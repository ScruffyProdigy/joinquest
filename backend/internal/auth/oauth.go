package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/store"
	"golang.org/x/oauth2"
)

const oauthStateTTL = 10 * time.Minute

var oauthHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
	Transport: &http.Transport{
		ForceAttemptHTTP2:     false,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

var (
	ErrOAuthNotConfigured     = errors.New("This sign-in provider is not available right now.")
	ErrOAuthInvalidState      = errors.New("Sign-in session expired. Please try again.")
	ErrOAuthProviderError     = errors.New("Could not sign in with that provider. Please try again.")
	ErrIdentityAlreadyLinked  = errors.New("That account is already linked to another JoinQuest profile.")
	ErrOAuthMergeConfirmation = errors.New("This account belongs to another JoinQuest profile. Confirm the merge to continue.")
)

type OAuthProfile struct {
	Subject       string
	Email         *string
	EmailVerified bool
	DisplayName   string
}

type oauthProviderConfig struct {
	name         string
	clientID     string
	clientSecret string
	scopes       []string
	authURL      string
	tokenURL     string
	userInfoURL  string
}

func oauthCallbackURL(provider string) string {
	return LobbyIssuer() + "/auth/oauth/" + strings.ToLower(provider) + "/callback"
}

func oauthProviderFromName(name string) (*oauthProviderConfig, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "google":
		cfg := oauthProviderConfig{
			name:         "google",
			clientID:     strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID")),
			clientSecret: strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")),
			scopes:       []string{"openid", "email", "profile"},
			authURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			tokenURL:     "https://oauth2.googleapis.com/token",
			userInfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
		}
		if cfg.clientID == "" || cfg.clientSecret == "" {
			return nil, ErrOAuthNotConfigured
		}
		return &cfg, nil
	case "discord":
		cfg := oauthProviderConfig{
			name:         "discord",
			clientID:     strings.TrimSpace(os.Getenv("DISCORD_OAUTH_CLIENT_ID")),
			clientSecret: strings.TrimSpace(os.Getenv("DISCORD_OAUTH_CLIENT_SECRET")),
			scopes:       []string{"identify"},
			authURL:      "https://discord.com/api/oauth2/authorize",
			tokenURL:     "https://discord.com/api/oauth2/token",
			userInfoURL:  "https://discord.com/api/users/@me",
		}
		if cfg.clientID == "" || cfg.clientSecret == "" {
			return nil, ErrOAuthNotConfigured
		}
		return &cfg, nil
	default:
		return nil, ErrOAuthNotConfigured
	}
}

func (cfg *oauthProviderConfig) oauth2Config() *oauth2.Config {
	endpoint := oauth2.Endpoint{
		AuthURL:  cfg.authURL,
		TokenURL: cfg.tokenURL,
	}
	if cfg.name == "discord" {
		endpoint.AuthStyle = oauth2.AuthStyleInParams
	}
	return &oauth2.Config{
		ClientID:     cfg.clientID,
		ClientSecret: cfg.clientSecret,
		RedirectURL:  oauthCallbackURL(cfg.name),
		Scopes:       cfg.scopes,
		Endpoint:     endpoint,
	}
}

func (cfg *oauthProviderConfig) authCodeURL(stateToken, pkceChallenge string) string {
	oauthCfg := cfg.oauth2Config()
	var opts []oauth2.AuthCodeOption
	if cfg.name == "google" {
		opts = append(opts, oauth2.AccessTypeOnline)
	}
	if challenge := strings.TrimSpace(pkceChallenge); challenge != "" && cfg.name != "discord" {
		opts = append(opts,
			oauth2.SetAuthURLParam("code_challenge", challenge),
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		)
	}
	return oauthCfg.AuthCodeURL(stateToken, opts...)
}

func (cfg *oauthProviderConfig) exchangeCode(ctx context.Context, code, pkceVerifier string) (*oauth2.Token, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrOAuthProviderError
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, oauthHTTPClient)
	oauthCfg := cfg.oauth2Config()
	if cfg.name == "discord" {
		return exchangeDiscordCode(ctx, oauthCfg, code)
	}
	return oauthCfg.Exchange(ctx, code)
}

func exchangeDiscordCode(ctx context.Context, oauthCfg *oauth2.Config, code string) (*oauth2.Token, error) {
	start := time.Now()
	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		log.Printf("auth: discord token exchange failed after %s: %v", time.Since(start), err)
		return nil, ErrOAuthProviderError
	}
	log.Printf("auth: discord token exchange ok in %s", time.Since(start))
	return token, nil
}

// EnabledOAuthProviders returns provider names configured in the environment.
func EnabledOAuthProviders() []string {
	var providers []string
	if id := strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID")); id != "" {
		if secret := strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")); secret != "" {
			providers = append(providers, "google")
		}
	}
	if id := strings.TrimSpace(os.Getenv("DISCORD_OAUTH_CLIENT_ID")); id != "" {
		if secret := strings.TrimSpace(os.Getenv("DISCORD_OAUTH_CLIENT_SECRET")); secret != "" {
			providers = append(providers, "discord")
		}
	}
	return providers
}

func (s *Service) OAuthStartURL(ctx context.Context, provider string, mode OAuthMode, confirmMerge bool, next string) (string, error) {
	cfg, err := oauthProviderFromName(provider)
	if err != nil {
		return "", err
	}

	state := OAuthState{
		Provider:     cfg.name,
		Mode:         mode,
		ConfirmMerge: confirmMerge,
		Next:         NormalizeOAuthNextKey(next),
	}
	if mode == OAuthModeLink {
		user, err := s.GetAuthenticatedUser(ctx)
		if err != nil {
			return "", err
		}
		if user == nil {
			return "", ErrAuthenticationRequired
		}
		state.UserID = user.ID
	}

	stateID, err := s.oauthStateStore.Save(ctx, state, "", oauthStateTTL)
	if err != nil {
		return "", err
	}
	return cfg.authCodeURL(stateID, ""), nil
}

func (s *Service) resolveOAuthState(ctx context.Context, stateToken string) (OAuthState, string, error) {
	stateToken = strings.TrimSpace(stateToken)
	if stateToken == "" {
		return OAuthState{}, "", ErrOAuthInvalidState
	}
	if _, err := uuid.Parse(stateToken); err == nil {
		return s.oauthStateStore.Load(ctx, stateToken)
	}
	state, err := s.signer.VerifyOAuthState(stateToken)
	return state, "", err
}

func (s *Service) CompleteOAuthCallback(ctx context.Context, provider, code, stateToken string) (*store.User, string, *OAuthMergeRequired, OAuthState, error) {
	start := time.Now()
	logStep := func(step string) {
		log.Printf("auth: oauth %s callback %s (%s elapsed)", provider, step, time.Since(start))
	}

	cfg, err := oauthProviderFromName(provider)
	if err != nil {
		return nil, "", nil, OAuthState{}, err
	}
	state, _, err := s.resolveOAuthState(ctx, stateToken)
	logStep("state loaded")
	if err != nil {
		return nil, "", nil, OAuthState{}, ErrOAuthInvalidState
	}
	if state.Provider != cfg.name {
		return nil, "", nil, state, ErrOAuthInvalidState
	}

	token, err := cfg.exchangeCode(ctx, code, "")
	logStep("token exchanged")
	if err != nil {
		return nil, "", nil, state, ErrOAuthProviderError
	}

	profile, err := fetchOAuthProfile(ctx, cfg, token.AccessToken)
	logStep("profile fetched")
	if err != nil {
		return nil, "", nil, state, err
	}
	if strings.TrimSpace(profile.Subject) == "" {
		return nil, "", nil, state, ErrOAuthProviderError
	}

	var user *store.User
	var sessionToken string
	var merge *OAuthMergeRequired

	switch state.Mode {
	case OAuthModeSignIn:
		user, sessionToken, err = s.resolveOAuthSignIn(ctx, cfg.name, code, profile)
	case OAuthModeLink:
		user, sessionToken, merge, err = s.finishOAuthLink(ctx, state.UserID, cfg.name, profile, state.ConfirmMerge)
	default:
		return nil, "", nil, OAuthState{}, ErrOAuthInvalidState
	}
	logStep("session created")

	if err == nil && merge == nil {
		if _, parseErr := uuid.Parse(strings.TrimSpace(stateToken)); parseErr == nil {
			if delErr := s.oauthStateStore.Delete(context.Background(), stateToken); delErr != nil {
				log.Printf("auth: oauth state delete failed: %v", delErr)
			}
		}
	}

	return user, sessionToken, merge, state, err
}

type OAuthMergeRequired struct {
	Provider               string
	MergeSourceDisplayName string
}

func (s *Service) oauthDBContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := oauthDBTimeout
	if deadline, ok := parent.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (s *Service) sessionForExistingIdentity(ctx context.Context, provider, subject string) (*store.User, string, error) {
	dbCtx, cancel := s.oauthDBContext(ctx)
	defer cancel()
	return s.sessionForExistingIdentityWithDB(ctx, dbCtx, provider, subject)
}

func (s *Service) sessionForExistingIdentityWithDB(ctx context.Context, dbCtx context.Context, provider, subject string) (*store.User, string, error) {
	existing, err := s.store.GetUserIdentityByProviderSubject(dbCtx, provider, subject)
	if err != nil {
		return nil, "", err
	}
	return s.sessionForUserID(dbCtx, existing.UserID)
}

func (s *Service) resolveOAuthSignIn(ctx context.Context, provider, code string, profile OAuthProfile) (*store.User, string, error) {
	if code != "" && s.oauthCodeStore != nil {
		if userID, ok, err := s.oauthCodeStore.GetCompleted(ctx, provider, code); err == nil && ok {
			if id, parseErr := uuid.Parse(userID); parseErr == nil {
				dbCtx, cancel := s.oauthDBContext(ctx)
				user, token, sessionErr := s.sessionForUserID(dbCtx, id)
				cancel()
				if sessionErr == nil {
					return user, token, nil
				}
			}
		}
		claimed, err := s.oauthCodeStore.TryClaim(ctx, provider, code)
		if err != nil {
			log.Printf("auth: oauth code claim failed: %v", err)
		} else if !claimed {
			if user, token, waitErr := s.waitForDuplicateOAuthCallback(ctx, provider, code, profile.Subject); waitErr == nil {
				return user, token, nil
			}
		}
	}

	user, token, err := s.finishOAuthSignIn(ctx, provider, profile)
	if err == nil && user != nil && code != "" && s.oauthCodeStore != nil {
		if setErr := s.oauthCodeStore.SetCompleted(context.Background(), provider, code, user.ID.String()); setErr != nil {
			log.Printf("auth: oauth code completion store failed: %v", setErr)
		}
	}
	return user, token, err
}

func (s *Service) waitForDuplicateOAuthCallback(ctx context.Context, provider, code, subject string) (*store.User, string, error) {
	for i := 0; i < 50; i++ {
		if userID, ok, err := s.oauthCodeStore.GetCompleted(ctx, provider, code); err == nil && ok {
			if id, parseErr := uuid.Parse(userID); parseErr == nil {
				dbCtx, cancel := s.oauthDBContext(ctx)
				user, token, sessionErr := s.sessionForUserID(dbCtx, id)
				cancel()
				if sessionErr == nil {
					return user, token, nil
				}
			}
		}
		if user, token, err := s.sessionForExistingIdentity(ctx, provider, subject); err == nil {
			return user, token, nil
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil, "", errors.New("auth: duplicate oauth callback timed out")
}

func oauthLinkRecoverable(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, store.ErrIdentityAlreadyLinked) ||
		errors.Is(err, store.ErrIdentityLinkBlocked) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

func (s *Service) finishOAuthSignIn(ctx context.Context, provider string, profile OAuthProfile) (*store.User, string, error) {
	start := time.Now()
	logOAuthStep := func(step string) {
		log.Printf("auth: oauth sign-in %s (%s elapsed)", step, time.Since(start))
	}

	dbCtx, dbCancel := s.oauthDBContext(ctx)
	defer dbCancel()

	if err := s.store.Ping(dbCtx); err != nil {
		log.Printf("auth: oauth sign-in db ping failed: %v", err)
	}

	if user, token, err := s.sessionForExistingIdentityWithDB(ctx, dbCtx, provider, profile.Subject); err == nil {
		logOAuthStep("existing identity login")
		return user, token, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, "", err
	}
	logOAuthStep("identity lookup")

	if provider != "discord" && profile.EmailVerified && profile.Email != nil {
		logOAuthStep("email path lookup")
		ownerID, err := s.store.ResolveUserIDByEmail(dbCtx, *profile.Email)
		if err == nil {
			logOAuthStep("email path owner found")
			_, linkErr := s.store.CreateUserIdentity(dbCtx, ownerID, provider, profile.Subject, profile.Email)
			if linkErr != nil && !errors.Is(linkErr, store.ErrIdentityAlreadyLinked) && !oauthLinkRecoverable(linkErr) {
				return nil, "", linkErr
			}
			if oauthLinkRecoverable(linkErr) {
				if user, token, existingErr := s.sessionForExistingIdentityWithDB(ctx, dbCtx, provider, profile.Subject); existingErr == nil {
					logOAuthStep("email path reused existing identity")
					return user, token, nil
				}
			} else {
				logOAuthStep("email identity linked")
			}
			return s.sessionForUserID(dbCtx, ownerID)
		}
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, "", err
		}
	}

	logOAuthStep("creating oauth account")
	user, err := s.store.CompleteOAuthSignUp(dbCtx, provider, profile.Subject, profile.DisplayName, profileEmail(profile))
	if err != nil {
		if oauthLinkRecoverable(err) {
			if existingUser, token, existingErr := s.sessionForExistingIdentityWithDB(ctx, dbCtx, provider, profile.Subject); existingErr == nil {
				logOAuthStep("sign-up race reused existing identity")
				return existingUser, token, nil
			}
		}
		return nil, "", err
	}
	logOAuthStep("oauth account created")

	if profile.EmailVerified && profile.Email != nil {
		if err := s.attachVerifiedOAuthEmail(dbCtx, user.ID, profile); err != nil {
			return nil, "", err
		}
	}

	return s.sessionForUserID(dbCtx, user.ID)
}

func (s *Service) finishOAuthLink(ctx context.Context, targetUserID uuid.UUID, provider string, profile OAuthProfile, confirmMerge bool) (*store.User, string, *OAuthMergeRequired, error) {
	existing, err := s.store.GetUserIdentityByProviderSubject(ctx, provider, profile.Subject)
	if err == nil {
		if existing.UserID == targetUserID {
			user, token, err := s.sessionForUserID(ctx, targetUserID)
			return user, token, nil, err
		}
		if !confirmMerge {
			source, err := s.store.GetUserByID(ctx, existing.UserID)
			if err != nil {
				return nil, "", nil, err
			}
			return nil, "", &OAuthMergeRequired{
				Provider:               provider,
				MergeSourceDisplayName: strings.TrimSpace(source.DisplayName),
			}, nil
		}
		if err := s.store.MergeUserInto(ctx, existing.UserID, targetUserID); err != nil {
			return nil, "", nil, err
		}
		user, token, err := s.sessionForUserID(ctx, targetUserID)
		return user, token, nil, err
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, "", nil, err
	}

	if _, err := s.store.CreateUserIdentity(ctx, targetUserID, provider, profile.Subject, profileEmail(profile)); err != nil {
		return nil, "", nil, err
	}
	if err := s.attachVerifiedOAuthEmail(ctx, targetUserID, profile); err != nil {
		return nil, "", nil, err
	}
	user, token, err := s.sessionForUserID(ctx, targetUserID)
	return user, token, nil, err
}

func (s *Service) attachVerifiedOAuthEmail(ctx context.Context, userID uuid.UUID, profile OAuthProfile) error {
	if !profile.EmailVerified || profile.Email == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*profile.Email))
	if normalized == "" {
		return nil
	}
	_, err := s.store.AddVerifiedUserEmail(ctx, userID, normalized, false)
	if errors.Is(err, store.ErrEmailAlreadyLinked) {
		return nil
	}
	return err
}

func (s *Service) sessionForUserID(ctx context.Context, userID uuid.UUID) (*store.User, string, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if err := s.store.UpdateUserLastLogin(ctx, user.ID); err != nil {
		return nil, "", err
	}
	sessionToken, err := s.signer.SignUserToken(user.ID, s.sessionTTL)
	if err != nil {
		return nil, "", err
	}
	return user, sessionToken, nil
}

func profileEmail(profile OAuthProfile) *string {
	if profile.Email == nil {
		return nil
	}
	normalized := strings.ToLower(strings.TrimSpace(*profile.Email))
	if normalized == "" {
		return nil
	}
	return &normalized
}

func fetchOAuthProfile(ctx context.Context, cfg *oauthProviderConfig, accessToken string) (OAuthProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.userInfoURL, nil)
	if err != nil {
		return OAuthProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "JoinQuest/1.0 (+https://joinquest.cc)")

	start := time.Now()
	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		log.Printf("auth: oauth profile request failed after %s: %v", time.Since(start), err)
		return OAuthProfile{}, ErrOAuthProviderError
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		log.Printf("auth: oauth profile request status=%d after %s body=%s", resp.StatusCode, time.Since(start), string(body))
		return OAuthProfile{}, ErrOAuthProviderError
	}
	log.Printf("auth: oauth profile request ok in %s", time.Since(start))

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return OAuthProfile{}, ErrOAuthProviderError
	}

	switch cfg.name {
	case "google":
		return parseGoogleProfile(body)
	case "discord":
		return parseDiscordProfile(body)
	default:
		return OAuthProfile{}, ErrOAuthNotConfigured
	}
}

func parseGoogleProfile(body []byte) (OAuthProfile, error) {
	var payload struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return OAuthProfile{}, ErrOAuthProviderError
	}
	profile := OAuthProfile{
		Subject:       payload.Sub,
		EmailVerified: payload.EmailVerified,
		DisplayName:   strings.TrimSpace(payload.Name),
	}
	if email := strings.TrimSpace(payload.Email); email != "" {
		profile.Email = &email
	}
	return profile, nil
}

func parseDiscordProfile(body []byte) (OAuthProfile, error) {
	var payload struct {
		ID         json.RawMessage `json:"id"`
		Email      string          `json:"email"`
		Verified   bool            `json:"verified"`
		Username   string          `json:"username"`
		GlobalName string          `json:"global_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return OAuthProfile{}, ErrOAuthProviderError
	}
	subject := discordSnowflakeString(payload.ID)
	if subject == "" {
		return OAuthProfile{}, ErrOAuthProviderError
	}
	displayName := strings.TrimSpace(payload.GlobalName)
	if displayName == "" {
		displayName = strings.TrimSpace(payload.Username)
	}
	profile := OAuthProfile{
		Subject:       subject,
		EmailVerified: payload.Verified,
		DisplayName:   displayName,
	}
	if email := strings.TrimSpace(payload.Email); email != "" {
		profile.Email = &email
	}
	return profile, nil
}

func discordSnowflakeString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}
	var asNumber json.Number
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return strings.TrimSpace(asNumber.String())
	}
	return ""
}

// RemoveLinkedIdentity removes an OAuth identity from the account.
func (s *Service) RemoveLinkedIdentity(ctx context.Context, userID, identityID uuid.UUID) (*store.User, error) {
	if err := s.store.RemoveUserIdentity(ctx, userID, identityID); err != nil {
		if errors.Is(err, store.ErrLastSignInMethod) {
			return nil, ErrLastSignInMethod
		}
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrIdentityNotLinked
		}
		return nil, err
	}
	return s.store.GetUserByID(ctx, userID)
}

func oauthRedirectURL(values url.Values) string {
	base := LobbyPublicURL()
	if len(values) == 0 {
		return base + "/"
	}
	return base + "/auth/oauth/complete?" + values.Encode()
}
