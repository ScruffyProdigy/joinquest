package auth

import (
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultCookieName = "lobby_session"

// CookieConfig controls session cookie behavior.
type CookieConfig struct {
	Name     string
	Secure   bool
	SameSite http.SameSite
	MaxAge   int
}

// CookieConfigFromEnv loads cookie settings from environment variables.
func CookieConfigFromEnv() CookieConfig {
	name := os.Getenv("SESSION_COOKIE_NAME")
	if name == "" {
		name = defaultCookieName
	}

	secure := strings.EqualFold(os.Getenv("SESSION_COOKIE_SECURE"), "true")
	if IsProductionEnv() {
		secure = true
	}

	return CookieConfig{
		Name:     name,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	}
}

// TokenFromCookie reads a JWT from the session cookie only. Sliding renewal is
// cookie-only: a Bearer caller holds its own credential and has no cookie jar.
func TokenFromCookie(r *http.Request) string {
	cfg := CookieConfigFromEnv()
	if cookie, err := r.Cookie(cfg.Name); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

// TokenFromRequest reads a JWT from the session cookie or Authorization header.
func TokenFromRequest(r *http.Request) string {
	if token := TokenFromCookie(r); token != "" {
		return token
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}

	return ""
}

// SetSessionCookie writes the session JWT to the response.
func SetSessionCookie(w http.ResponseWriter, token string, cfg CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Name,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
		MaxAge:   cfg.MaxAge,
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter, cfg CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
		MaxAge:   -1,
	})
}

// slideSessionCookie re-issues the session cookie once a request arrives past the
// halfway mark of the token's life, so an active player never expires out of their
// own identity mid-play (JQ-86). A guest has no credential to re-authenticate with,
// so at expiry their user row is orphaned and their history with every integrated
// game is gone — sliding is what keeps that from happening to someone still playing.
//
// It returns the token the request should be treated as carrying: the renewed one
// when a renewal happened, otherwise the original. The new token is minted for the
// same lifetime the old one had, so SESSION_TTL stays the single source of truth.
func slideSessionCookie(w http.ResponseWriter, r *http.Request, signer *Signer, session UserSession, token string) string {
	if w == nil || !session.PastHalfLife() {
		return token
	}
	// Only a browser session cookie can be slid; a Bearer caller has no cookie jar.
	if TokenFromCookie(r) != token {
		return token
	}
	renewed, err := signer.SignUserToken(session.UserID, session.Lifetime())
	if err != nil {
		return token
	}
	SetSessionCookie(w, renewed, CookieConfigFromEnv())
	return renewed
}

// SessionJWTMiddleware attaches user context from the session cookie without database lookups.
func SessionJWTMiddleware(signer *Signer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if token := TokenFromRequest(r); token != "" {
			if session, err := signer.VerifyUserSession(token); err == nil {
				token = slideSessionCookie(w, r, signer, session, token)
				ctx = WithUserID(ctx, session.UserID.String())
				ctx = WithSessionToken(ctx, token)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Middleware authenticates requests and attaches user context.
func Middleware(signer *Signer, apiKeys DeveloperAPIKeyVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithResponseWriter(r.Context(), w)
		ctx = WithUserAgent(ctx, r.UserAgent())

		if token := TokenFromRequest(r); token != "" {
			if MatchesGameServiceToken(token) {
				ctx = WithGameServiceAuth(ctx)
				if gameID, err := ParseGameServiceToken(token); err == nil {
					ctx = WithGameServiceGameID(ctx, gameID.String())
				}
			} else if MatchesDeveloperAPIKey(token) && apiKeys != nil {
				if userID, err := apiKeys.VerifyDeveloperAPIKey(r.Context(), token); err == nil {
					ctx = WithUserID(ctx, userID.String())
				}
			} else if session, err := signer.VerifyUserSession(token); err == nil {
				token = slideSessionCookie(w, r, signer, session, token)
				ctx = WithUserID(ctx, session.UserID.String())
				ctx = WithSessionToken(ctx, token)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
