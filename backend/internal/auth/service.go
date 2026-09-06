package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/email"
	"github.com/scruffyprodigy/playhub/internal/store"
)

var (
	ErrInvalidEmail         = errors.New("auth: invalid email address")
	ErrInvalidMagicLink     = errors.New("Invalid or expired sign-in link. Request a new sign-in email.")
	ErrInvalidLoginCode     = errors.New("Invalid or expired code. Try again or use the sign-in link in your email.")
	ErrMagicLinkRateLimit   = errors.New("Too many sign-in emails requested. Please wait and try again.")
	ErrTooManyLoginAttempts = errors.New("Too many incorrect codes. Request a new sign-in email.")
	ErrSignInEmailNotSent   = errors.New("Could not send sign-in email. Please try again in a few minutes.")
)

const (
	maxMagicLinksPerEmailPerHour = 5
	magicLinkRateWindow          = time.Hour
)

// Service handles authentication workflows.
type Service struct {
	store            *store.Store
	signer           *Signer
	mailer           email.Sender
	oauthStateStore  OAuthStateStore
	oauthCodeStore   OAuthCodeStore
	cookie           CookieConfig
	magicLinkTTL     time.Duration
	sessionTTL       time.Duration
	magicLinkBaseURL string
}

// NewService creates an authentication service backed by the store and JWT signer.
func NewService(st *store.Store, signer *Signer) (*Service, error) {
	if st == nil {
		return nil, errors.New("auth: store is required")
	}
	if signer == nil {
		return nil, errors.New("auth: signer is required")
	}

	mailer, err := email.SenderFromEnv()
	if err != nil {
		return nil, err
	}

	return NewServiceWithMailer(st, signer, mailer)
}

// NewServiceWithMailer creates an authentication service with an explicit mailer.
func NewServiceWithMailer(st *store.Store, signer *Signer, mailer email.Sender) (*Service, error) {
	if st == nil {
		return nil, errors.New("auth: store is required")
	}
	if signer == nil {
		return nil, errors.New("auth: signer is required")
	}
	if mailer == nil {
		return nil, errors.New("auth: mailer is required")
	}

	return newService(st, signer, mailer), nil
}

func newService(st *store.Store, signer *Signer, mailer email.Sender) *Service {
	oauthStateStore, err := NewOAuthStateStoreFromEnv()
	if err != nil {
		log.Printf("auth: oauth state store unavailable, using in-memory fallback: %v", err)
		oauthStateStore = newMemoryOAuthStateStore()
	}
	oauthCodeStore, err := NewOAuthCodeStoreFromEnv()
	if err != nil {
		log.Printf("auth: oauth code store unavailable, using in-memory fallback: %v", err)
		oauthCodeStore = newMemoryOAuthCodeStore()
	}
	return &Service{
		store:            st,
		signer:           signer,
		mailer:           mailer,
		oauthStateStore:  oauthStateStore,
		oauthCodeStore:   oauthCodeStore,
		cookie:           CookieConfigFromEnv(),
		magicLinkTTL:     durationFromEnv("MAGIC_LINK_TTL", 15*time.Minute),
		sessionTTL:       durationFromEnv("SESSION_TTL", 7*24*time.Hour),
		magicLinkBaseURL: strings.TrimSpace(os.Getenv("MAGIC_LINK_BASE_URL")),
	}
}

// Signer returns the JWT signer used by the service.
func (s *Service) Signer() *Signer {
	return s.signer
}

// CookieConfig returns the session cookie configuration.
func (s *Service) CookieConfig() CookieConfig {
	return s.cookie
}

// RequestMagicLink creates a sign-in email with a magic link and 6-digit code.
func (s *Service) RequestMagicLink(ctx context.Context, emailAddr string) error {
	normalized, err := normalizeEmail(emailAddr)
	if err != nil {
		return err
	}

	since := time.Now().Add(-magicLinkRateWindow)
	count, err := s.store.CountRecentMagicLinksByEmail(ctx, normalized, since)
	if err != nil {
		return err
	}
	if count >= maxMagicLinksPerEmailPerHour {
		return ErrMagicLinkRateLimit
	}

	code, err := generateLoginCode()
	if err != nil {
		return err
	}

	token := uuid.NewString()
	link, err := s.store.CreateMagicLink(ctx, store.CreateMagicLinkParams{
		Email:     normalized,
		TokenHash: hashMagicLinkToken(token),
		CodeHash:  hashLoginCode(code),
		ExpiresAt: time.Now().Add(s.magicLinkTTL),
	})
	if err != nil {
		return err
	}

	if err := s.deliverLoginEmail(ctx, normalized, token, code); err != nil {
		if delErr := s.store.DeleteMagicLink(ctx, link.ID); delErr != nil {
			log.Printf("auth: rollback magic link %s after email failure: %v", link.ID, delErr)
		}
		return err
	}
	return nil
}

// CompleteMagicLogin validates a magic link token and returns the user plus session JWT.
func (s *Service) CompleteMagicLogin(ctx context.Context, token string) (*store.User, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, "", ErrInvalidMagicLink
	}
	link, err := s.store.ConsumeMagicLinkByTokenHash(ctx, hashMagicLinkToken(token), time.Now())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", ErrInvalidMagicLink
		}
		return nil, "", err
	}
	return s.finishLogin(ctx, link)
}

// CompleteLoginCode validates a 6-digit code for the given email.
func (s *Service) CompleteLoginCode(ctx context.Context, emailAddr, code string) (*store.User, string, error) {
	normalized, err := normalizeEmail(emailAddr)
	if err != nil {
		return nil, "", err
	}

	now := time.Now()
	link, err := s.store.GetLatestValidMagicLinkByEmail(ctx, normalized, now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", ErrInvalidLoginCode
		}
		return nil, "", err
	}

	if !verifyLoginCode(strings.TrimSpace(code), link.CodeHash) {
		if incErr := s.store.IncrementMagicLinkFailedAttempts(ctx, link.ID); incErr != nil {
			return nil, "", incErr
		}
		if link.FailedAttempts+1 >= store.MaxLoginCodeAttempts {
			return nil, "", ErrTooManyLoginAttempts
		}
		return nil, "", ErrInvalidLoginCode
	}

	link, err = s.store.ConsumeMagicLinkByID(ctx, link.ID, now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", ErrInvalidLoginCode
		}
		return nil, "", err
	}
	return s.finishLogin(ctx, link)
}

func (s *Service) finishLogin(ctx context.Context, link *store.MagicLink) (*store.User, string, error) {
	if link.UserID != nil {
		return nil, "", ErrInvalidMagicLink
	}

	user, err := s.findOrCreateUser(ctx, link.Email)
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

// GetAuthenticatedUser loads the current user from context, if any.
func (s *Service) GetAuthenticatedUser(ctx context.Context) (*store.User, error) {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, nil
	}

	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("auth: invalid user id in context: %w", err)
	}

	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

func (s *Service) findOrCreateUser(ctx context.Context, email string) (*store.User, error) {
	user, err := s.store.GetUserByEmail(ctx, email)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	return s.store.CreateUser(ctx, store.CreateUserParams{
		Email: email,
	})
}

// Logout clears the session cookie.
func (s *Service) Logout(ctx context.Context) {
	if writer, ok := ResponseWriterFromContext(ctx); ok {
		ClearSessionCookie(writer, s.cookie)
	}
}

func (s *Service) deliverLoginEmail(ctx context.Context, recipient, token, code string) error {
	link := s.magicLinkURL(token)
	if link == "" {
		return fmt.Errorf("%w: MAGIC_LINK_BASE_URL is not configured", ErrSignInEmailNotSent)
	}
	if err := s.mailer.SendMagicLink(ctx, email.MagicLinkEmail{
		To:   recipient,
		Link: link,
		Code: code,
		TTL:  s.magicLinkTTL,
	}); err != nil {
		log.Printf("auth: failed to send sign-in email to %s: %v", recipient, err)
		return fmt.Errorf("%w: %v", ErrSignInEmailNotSent, err)
	}
	log.Printf("auth: sent sign-in email to %s", recipient)
	return nil
}

func (s *Service) deliverLinkEmail(ctx context.Context, recipient, token, code string) error {
	link := s.linkEmailURL(token)
	if link == "" {
		return fmt.Errorf("%w: MAGIC_LINK_BASE_URL is not configured", ErrSignInEmailNotSent)
	}
	if err := s.mailer.SendMagicLink(ctx, email.MagicLinkEmail{
		To:   recipient,
		Link: link,
		Code: code,
		TTL:  s.magicLinkTTL,
	}); err != nil {
		log.Printf("auth: failed to send link email to %s: %v", recipient, err)
		return fmt.Errorf("%w: %v", ErrSignInEmailNotSent, err)
	}
	log.Printf("auth: sent link verification email to %s", recipient)
	return nil
}

func (s *Service) linkEmailURL(token string) string {
	base := strings.Replace(s.magicLinkBaseURL, "/auth/complete", "/auth/link", 1)
	if base == s.magicLinkBaseURL {
		if strings.Contains(s.magicLinkBaseURL, "{token}") {
			return strings.ReplaceAll(s.magicLinkBaseURL, "/auth/complete", "/auth/link")
		}
		if strings.Contains(base, "?") {
			return strings.Replace(base, "/auth/complete", "/auth/link", 1) + token
		}
		return strings.TrimRight(strings.Replace(s.magicLinkBaseURL, "/auth/complete", "/auth/link", 1), "/") + "?token=" + token
	}
	if strings.Contains(base, "{token}") {
		return strings.ReplaceAll(base, "{token}", token)
	}
	if strings.Contains(base, "%s") {
		return fmt.Sprintf(base, token)
	}
	if strings.Contains(base, "?") {
		return base + token
	}
	return strings.TrimRight(base, "/") + "?token=" + token
}

func (s *Service) magicLinkURL(token string) string {
	if s.magicLinkBaseURL == "" {
		return ""
	}
	if strings.Contains(s.magicLinkBaseURL, "{token}") {
		return strings.ReplaceAll(s.magicLinkBaseURL, "{token}", token)
	}
	if strings.Contains(s.magicLinkBaseURL, "%s") {
		return fmt.Sprintf(s.magicLinkBaseURL, token)
	}
	if strings.Contains(s.magicLinkBaseURL, "?") {
		return s.magicLinkBaseURL + token
	}
	return strings.TrimRight(s.magicLinkBaseURL, "/") + "?token=" + token
}

func normalizeEmail(email string) (string, error) {
	address, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil {
		return "", ErrInvalidEmail
	}
	return strings.ToLower(address.Address), nil
}

func durationFromEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
