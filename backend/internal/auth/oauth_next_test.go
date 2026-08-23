package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNormalizeOAuthNextKeyAcceptsKnownKeys(t *testing.T) {
	for _, key := range []string{"dev-ai", "dev-manual"} {
		if got := NormalizeOAuthNextKey(key); got != key {
			t.Fatalf("NormalizeOAuthNextKey(%q) = %q, want %q", key, got, key)
		}
	}
}

func TestNormalizeOAuthNextKeyRejectsEverythingElse(t *testing.T) {
	// The key is attacker-controllable via the query string. Anything that is not
	// an exact known key must be dropped so the redirect target stays a constant.
	hostile := []string{
		"",
		"   ",
		"unknown",
		"DEV-AI",
		"/developers",
		"//evil.com",
		"/\\evil.com",
		"https://evil.com",
		"javascript:alert(1)",
		"java\tscript:alert(1)",
		"dev-ai\n/evil",
		"dev-ai evil",
		"../../etc/passwd",
		"%2f%2fevil.com",
	}

	for _, key := range hostile {
		if got := NormalizeOAuthNextKey(key); got != "" {
			t.Fatalf("NormalizeOAuthNextKey(%q) = %q, want empty", key, got)
		}
	}
}

func TestResolveOAuthNextMapsKnownKeysToFixedPaths(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.test")

	cases := map[string]string{
		"dev-ai":     "https://joinquest.test/developers?path=ai",
		"dev-manual": "https://joinquest.test/developers?path=manual",
	}

	for key, want := range cases {
		if got := ResolveOAuthNext(key); got != want {
			t.Fatalf("ResolveOAuthNext(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestResolveOAuthNextFallsBackHomeForUnknownKeys(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.test")

	want := "https://joinquest.test/"
	for _, key := range []string{"", "unknown", "javascript:alert(1)", "//evil.com"} {
		if got := ResolveOAuthNext(key); got != want {
			t.Fatalf("ResolveOAuthNext(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestOAuthStateStoreCarriesNextKey(t *testing.T) {
	store := newMemoryOAuthStateStore()
	state := OAuthState{
		Provider: "google",
		Mode:     OAuthModeSignIn,
		Next:     "dev-manual",
	}

	id, err := store.Save(context.Background(), state, "", time.Minute)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, _, err := store.Load(context.Background(), id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Next != "dev-manual" {
		t.Fatalf("loaded next = %q, want %q", loaded.Next, "dev-manual")
	}
}

func TestSignedOAuthStateCarriesNextKey(t *testing.T) {
	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	signer, err := loadOrCreateDevSigner("test-kid")
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	token, err := signer.SignOAuthState(OAuthState{
		Provider: "google",
		Mode:     OAuthModeSignIn,
		Next:     "dev-ai",
	}, time.Minute)
	if err != nil {
		t.Fatalf("SignOAuthState: %v", err)
	}

	state, err := signer.VerifyOAuthState(token)
	if err != nil {
		t.Fatalf("VerifyOAuthState: %v", err)
	}
	if state.Next != "dev-ai" {
		t.Fatalf("next = %q, want %q", state.Next, "dev-ai")
	}
}

// A tampered or stale state must not be able to smuggle a redirect target through.
func TestSignedOAuthStateDropsUnknownNextKey(t *testing.T) {
	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	signer, err := loadOrCreateDevSigner("test-kid")
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	token, err := signer.SignOAuthState(OAuthState{
		Provider: "google",
		Mode:     OAuthModeSignIn,
		Next:     "https://evil.com",
	}, time.Minute)
	if err != nil {
		t.Fatalf("SignOAuthState: %v", err)
	}

	state, err := signer.VerifyOAuthState(token)
	if err != nil {
		t.Fatalf("VerifyOAuthState: %v", err)
	}
	if state.Next != "" {
		t.Fatalf("next = %q, want empty", state.Next)
	}
}

// The start handler must carry the next key from the query string into OAuth state,
// so the callback can land the user back where they started.
func TestHandleOAuthStartStoresNextKey(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "secret")
	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")

	stateStore := newMemoryOAuthStateStore()
	service := &Service{oauthStateStore: stateStore}

	req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/start?next=dev-manual", nil)
	rec := httptest.NewRecorder()
	handleOAuthStart(rec, req, service, "google")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	stateID := oauthStateIDFromRedirect(t, rec.Header().Get("Location"))
	loaded, _, err := stateStore.Load(context.Background(), stateID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Next != "dev-manual" {
		t.Fatalf("stored next = %q, want %q", loaded.Next, "dev-manual")
	}
}

func TestHandleOAuthStartDropsHostileNextKey(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "secret")
	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")

	stateStore := newMemoryOAuthStateStore()
	service := &Service{oauthStateStore: stateStore}

	req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/start?next=https%3A%2F%2Fevil.com", nil)
	rec := httptest.NewRecorder()
	handleOAuthStart(rec, req, service, "google")

	stateID := oauthStateIDFromRedirect(t, rec.Header().Get("Location"))
	loaded, _, err := stateStore.Load(context.Background(), stateID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Next != "" {
		t.Fatalf("stored next = %q, want empty", loaded.Next)
	}
}

// The success page interpolates dest into an href and a JS location.replace, so it
// must only ever receive an already-resolved destination.
func TestWriteOAuthSignInSuccessUsesResolvedDestination(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.test")

	rec := httptest.NewRecorder()
	writeOAuthSignInSuccess(rec, "session-token", CookieConfig{}, ResolveOAuthNext("dev-manual"))

	body := rec.Body.String()
	if !strings.Contains(body, "https://joinquest.test/developers?path=manual") {
		t.Fatalf("body missing resolved destination: %s", body)
	}
	if strings.Contains(body, "dev-manual\"") {
		t.Fatalf("body leaked the raw next key: %s", body)
	}
}

func oauthStateIDFromRedirect(t *testing.T, location string) string {
	t.Helper()
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect %q: %v", location, err)
	}
	stateID := parsed.Query().Get("state")
	if stateID == "" {
		t.Fatalf("no state in redirect %q", location)
	}
	return stateID
}
