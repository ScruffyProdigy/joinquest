package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// signSessionTokenAged mints a session token that was issued `age` ago and expires
// `age + remaining` after that, so tests can place a token anywhere in its own life.
func signSessionTokenAged(t *testing.T, signer *Signer, userID uuid.UUID, age, remaining time.Duration) string {
	t.Helper()
	issued := time.Now().Add(-age)
	claims := jwt.MapClaims{
		"iss": LobbyIssuer(),
		"aud": sessionTokenAudience(),
		"typ": sessionTokenType,
		"sub": userID.String(),
		"jti": uuid.NewString(),
		"iat": issued.Unix(),
		"nbf": issued.Unix(),
		"exp": time.Now().Add(remaining).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = signer.kid
	raw, err := token.SignedString(signer.privateKey)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return raw
}

func requestWithSessionCookie(token string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	r.AddCookie(&http.Cookie{Name: CookieConfigFromEnv().Name, Value: token})
	return r
}

func sessionCookieFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	name := CookieConfigFromEnv().Name
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestVerifyUserSessionReportsLifetime(t *testing.T) {
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("LoadSignerFromEnv: %v", err)
	}
	userID := uuid.New()
	raw := signSessionTokenAged(t, signer, userID, 6*24*time.Hour, 24*time.Hour)

	sess, err := signer.VerifyUserSession(raw)
	if err != nil {
		t.Fatalf("VerifyUserSession: %v", err)
	}
	if sess.UserID != userID {
		t.Fatalf("UserID = %s, want %s", sess.UserID, userID)
	}
	if got := sess.Lifetime(); got < 6*24*time.Hour || got > 8*24*time.Hour {
		t.Fatalf("Lifetime = %s, want ~7d", got)
	}
	if !sess.PastHalfLife() {
		t.Fatal("a token with 1 of 7 days left should be past half life")
	}
}

func TestVerifyUserSessionFreshTokenIsNotPastHalfLife(t *testing.T) {
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("LoadSignerFromEnv: %v", err)
	}
	raw := signSessionTokenAged(t, signer, uuid.New(), time.Minute, 7*24*time.Hour)

	sess, err := signer.VerifyUserSession(raw)
	if err != nil {
		t.Fatalf("VerifyUserSession: %v", err)
	}
	if sess.PastHalfLife() {
		t.Fatal("a minute-old token should not be past half life")
	}
}

// An active guest must never expire out of their own identity, so a request made
// past the halfway mark of the token's life leaves with a renewed cookie (JQ-86).
func TestMiddlewareSlidesSessionCookiePastHalfLife(t *testing.T) {
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("LoadSignerFromEnv: %v", err)
	}
	userID := uuid.New()
	old := signSessionTokenAged(t, signer, userID, 6*24*time.Hour, 24*time.Hour)

	var seenUser string
	rec := httptest.NewRecorder()
	Middleware(signer, nil, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seenUser, _ = UserIDFromContext(r.Context())
	})).ServeHTTP(rec, requestWithSessionCookie(old))

	if seenUser != userID.String() {
		t.Fatalf("authenticated user = %q, want %s", seenUser, userID)
	}

	cookie := sessionCookieFrom(rec)
	if cookie == nil {
		t.Fatal("expected a renewed session cookie")
	}
	if cookie.Value == old {
		t.Fatal("expected a freshly minted token, got the same one back")
	}
	renewed, err := signer.VerifyUserSession(cookie.Value)
	if err != nil {
		t.Fatalf("renewed token does not verify: %v", err)
	}
	if renewed.UserID.String() != seenUser {
		t.Fatalf("renewed token subject = %s, want %s", renewed.UserID, seenUser)
	}
	if renewed.PastHalfLife() {
		t.Fatal("renewed token should start fresh")
	}
	// The renewal keeps the lifetime the token was originally issued with, so
	// SESSION_TTL stays the single source of truth for how long a session lasts.
	if got := renewed.Lifetime(); got < 6*24*time.Hour || got > 8*24*time.Hour {
		t.Fatalf("renewed lifetime = %s, want ~7d", got)
	}
}

func TestMiddlewareDoesNotSlideFreshSession(t *testing.T) {
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("LoadSignerFromEnv: %v", err)
	}
	fresh := signSessionTokenAged(t, signer, uuid.New(), time.Minute, 7*24*time.Hour)

	rec := httptest.NewRecorder()
	Middleware(signer, nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rec, requestWithSessionCookie(fresh))

	if cookie := sessionCookieFrom(rec); cookie != nil {
		t.Fatalf("expected no cookie rewrite for a fresh session, got %q", cookie.Value)
	}
}

// Bearer callers (game services, API keys, integration tests) hold their own
// credential; rewriting a browser cookie for them is meaningless and would leak a
// session token into a server-to-server response.
func TestMiddlewareDoesNotSlideBearerSession(t *testing.T) {
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("LoadSignerFromEnv: %v", err)
	}
	aged := signSessionTokenAged(t, signer, uuid.New(), 6*24*time.Hour, 24*time.Hour)

	r := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	r.Header.Set("Authorization", "Bearer "+aged)
	rec := httptest.NewRecorder()
	Middleware(signer, nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, r)

	if cookie := sessionCookieFrom(rec); cookie != nil {
		t.Fatalf("expected no cookie for a bearer caller, got %q", cookie.Value)
	}
}

func TestSessionJWTMiddlewareSlidesCookie(t *testing.T) {
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("LoadSignerFromEnv: %v", err)
	}
	old := signSessionTokenAged(t, signer, uuid.New(), 6*24*time.Hour, 24*time.Hour)

	rec := httptest.NewRecorder()
	SessionJWTMiddleware(signer, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(rec, requestWithSessionCookie(old))

	cookie := sessionCookieFrom(rec)
	if cookie == nil || cookie.Value == old {
		t.Fatal("expected a renewed session cookie")
	}
}
