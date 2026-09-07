package auth

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/scruffyprodigy/joinquest/internal/store"
)

const oauthCallbackTimeout = 90 * time.Second
const oauthDBTimeout = 15 * time.Second

// RegisterOAuthRoutes mounts browser OAuth start and callback handlers.
func RegisterOAuthRoutes(mux *http.ServeMux, service *Service, signer *Signer, dataStore *store.Store) {
	mux.Handle("/auth/oauth/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/auth/oauth/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		provider := parts[0]
		action := parts[1]

		switch action {
		case "start":
			Middleware(signer, dataStore, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleOAuthStart(w, r, service, provider)
			})).ServeHTTP(w, r)
		case "callback":
			SessionJWTMiddleware(signer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handleOAuthCallback(w, r, service, provider)
			})).ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}

func handleOAuthStart(w http.ResponseWriter, r *http.Request, service *Service, provider string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mode := OAuthModeSignIn
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("mode")), "link") {
		mode = OAuthModeLink
	}
	confirmMerge := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("confirm_merge")), "1") ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("confirm_merge")), "true")

	next := NormalizeOAuthNextKey(r.URL.Query().Get("next"))

	startURL, err := service.OAuthStartURL(r.Context(), provider, mode, confirmMerge, next)
	if err != nil {
		redirectOAuthError(w, r, err, mode)
		return
	}
	http.Redirect(w, r, startURL, http.StatusFound)
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request, service *Service, provider string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if errMsg := strings.TrimSpace(r.URL.Query().Get("error")); errMsg != "" {
		redirectOAuthError(w, r, ErrOAuthProviderError, OAuthModeSignIn)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	log.Printf("auth: oauth %s callback received code_present=%t state_len=%d", provider, code != "", len(state))
	ctx, cancel := context.WithTimeout(context.Background(), oauthCallbackTimeout)
	defer cancel()
	user, sessionToken, merge, oauthState, err := service.CompleteOAuthCallback(ctx, provider, code, state)
	if merge != nil {
		values := url.Values{}
		values.Set("oauth_merge", "1")
		values.Set("provider", merge.Provider)
		if merge.MergeSourceDisplayName != "" {
			values.Set("source", merge.MergeSourceDisplayName)
		}
		http.Redirect(w, r, accountRedirectURL(values), http.StatusFound)
		return
	}
	if err != nil {
		stats := service.store.DBStats()
		log.Printf("auth: oauth %s callback failed: %v (db pool open=%d inUse=%d idle=%d waitCount=%d waitDuration=%s)",
			provider, err, stats.OpenConnections, stats.InUse, stats.Idle, stats.WaitCount, stats.WaitDuration)
		redirectOAuthError(w, r, err, oauthState.Mode)
		return
	}

	if oauthState.Mode == OAuthModeLink {
		SetSessionCookie(w, sessionToken, service.CookieConfig())
		values := url.Values{}
		values.Set("linked", "1")
		if user != nil && !user.IsGuest {
			values.Set("saved", "1")
		}
		http.Redirect(w, r, accountRedirectURL(values), http.StatusFound)
		return
	}
	writeOAuthSignInSuccess(w, sessionToken, service.CookieConfig(), ResolveOAuthNext(oauthState.Next))
}

func writeOAuthSignInSuccess(w http.ResponseWriter, sessionToken string, cookie CookieConfig, dest string) {
	SetSessionCookie(w, sessionToken, cookie)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	escaped := html.EscapeString(dest)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="0;url=%s">
<title>Signing in…</title>
</head>
<body>
<p>Signing you in… <a href="%s">Continue</a></p>
<script>location.replace(%q);</script>
</body>
</html>`, escaped, escaped, dest)
}

func redirectOAuthError(w http.ResponseWriter, r *http.Request, err error, mode OAuthMode) {
	values := url.Values{}
	values.Set("error", oauthErrorCode(err))
	if mode == OAuthModeLink {
		http.Redirect(w, r, accountRedirectURL(values), http.StatusFound)
		return
	}
	http.Redirect(w, r, oauthRedirectURL(values), http.StatusFound)
}

func accountRedirectURL(values url.Values) string {
	base := LobbyPublicURL()
	if len(values) == 0 {
		return base + "/account"
	}
	return base + "/account?" + values.Encode()
}

func oauthErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrOAuthNotConfigured):
		return "not_configured"
	case errors.Is(err, ErrOAuthInvalidState):
		return "invalid_state"
	case errors.Is(err, ErrAuthenticationRequired):
		return "auth_required"
	case errors.Is(err, ErrMergeConfirmationRequired), errors.Is(err, ErrOAuthMergeConfirmation):
		return "merge_required"
	case errors.Is(err, ErrLastSignInMethod):
		return "last_method"
	default:
		return "provider_error"
	}
}
