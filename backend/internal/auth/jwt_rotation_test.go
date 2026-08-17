package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestJWKSHandlerIncludesAndRemovesRotationKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JWT_DEV_KEY_FILE", filepath.Join(dir, "test.pem"))
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	srv := httptest.NewServer(signer.JWKSHandler())
	t.Cleanup(srv.Close)

	kid, priv, cleanup, err := signer.AddRotationKey()
	if err != nil {
		t.Fatalf("AddRotationKey: %v", err)
	}

	jwks := fetchJWKSKeys(t, srv.URL)
	if len(jwks) != 2 {
		t.Fatalf("expected 2 keys after rotation, got %d", len(jwks))
	}
	rotEntry, ok := jwks[kid]
	if !ok {
		t.Fatalf("rotation kid %q missing from jwks: %+v", kid, jwks)
	}
	wantX := base64.RawURLEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
	if rotEntry["x"] != wantX {
		t.Fatalf("rotation key x mismatch: got %q want %q", rotEntry["x"], wantX)
	}

	cleanup()

	jwks = fetchJWKSKeys(t, srv.URL)
	if len(jwks) != 1 {
		t.Fatalf("expected 1 key after cleanup, got %d", len(jwks))
	}
	if _, ok := jwks[kid]; ok {
		t.Fatalf("rotation kid %q still present after cleanup", kid)
	}
}

func TestSignSeatTokenWithKeyVerifiesUnderRotationKid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JWT_DEV_KEY_FILE", filepath.Join(dir, "test.pem"))
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	kid, priv, cleanup, err := signer.AddRotationKey()
	if err != nil {
		t.Fatalf("AddRotationKey: %v", err)
	}
	t.Cleanup(cleanup)

	userID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	token, err := signer.SignSeatTokenWithKey(kid, priv, userID, "http://localhost:3001", "match-2", "a", "Rotated", time.Hour)
	if err != nil {
		t.Fatalf("SignSeatTokenWithKey: %v", err)
	}

	parsed, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		if token.Header["kid"] != kid {
			t.Fatalf("token kid: %v, want %v", token.Header["kid"], kid)
		}
		return priv.Public(), nil
	})
	if err != nil {
		t.Fatalf("verify rotation-signed token: %v", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		t.Fatal("invalid claims")
	}
	if claims["matchId"] != "match-2" || claims["seatKey"] != "a" {
		t.Fatalf("claims: %+v", claims)
	}
	if claims["iss"] != LobbyIssuer() {
		t.Fatalf("iss: %v", claims["iss"])
	}

	// The signer's own primary public key must NOT verify a rotation-signed token.
	_, err = jwt.Parse(token, func(token *jwt.Token) (any, error) {
		return signer.publicKey, nil
	})
	if err == nil {
		t.Fatal("expected primary key to fail verifying a rotation-signed token")
	}
}

// TestAddRotationKeyConcurrentSafe fires more concurrent AddRotationKey
// calls than maxRotationKeys allows, proving both that concurrent access is
// race-free (run with -race) and that the cap is enforced exactly — even
// under contention — with no overshoot from the check-then-insert race.
func TestAddRotationKeyConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JWT_DEV_KEY_FILE", filepath.Join(dir, "test.pem"))
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	const n = 20
	type addResult struct {
		cleanup func()
		err     error
	}
	resultsCh := make(chan addResult, n)
	for i := 0; i < n; i++ {
		go func() {
			_, _, cleanup, err := signer.AddRotationKey()
			resultsCh <- addResult{cleanup, err}
		}()
	}

	var cleanups []func()
	for i := 0; i < n; i++ {
		res := <-resultsCh
		if res.err == nil {
			cleanups = append(cleanups, res.cleanup)
		}
	}

	if len(cleanups) > maxRotationKeys {
		t.Fatalf("expected at most %d successful concurrent AddRotationKey calls, got %d", maxRotationKeys, len(cleanups))
	}

	srv := httptest.NewServer(signer.JWKSHandler())
	t.Cleanup(srv.Close)
	jwks := fetchJWKSKeys(t, srv.URL)
	if len(jwks) != len(cleanups)+1 {
		t.Fatalf("expected %d keys (primary + successful rotations), got %d", len(cleanups)+1, len(jwks))
	}

	for _, c := range cleanups {
		c()
	}

	jwks = fetchJWKSKeys(t, srv.URL)
	if len(jwks) != 1 {
		t.Fatalf("expected all rotation keys cleaned up, got %d keys", len(jwks))
	}
}

func TestAddRotationKeyEnforcesCap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JWT_DEV_KEY_FILE", filepath.Join(dir, "test.pem"))
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	var cleanups []func()
	for i := 0; i < maxRotationKeys; i++ {
		_, _, cleanup, err := signer.AddRotationKey()
		if err != nil {
			t.Fatalf("AddRotationKey %d: %v", i, err)
		}
		cleanups = append(cleanups, cleanup)
	}

	if _, _, _, err := signer.AddRotationKey(); err == nil {
		t.Fatal("expected AddRotationKey to fail once at cap")
	}

	cleanups[0]()
	if _, _, cleanup, err := signer.AddRotationKey(); err != nil {
		t.Fatalf("expected AddRotationKey to succeed after freeing a slot: %v", err)
	} else {
		cleanup()
	}

	for _, c := range cleanups[1:] {
		c()
	}
}

func fetchJWKSKeys(t *testing.T, url string) map[string]map[string]string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET jwks: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode jwks: %v", err)
	}
	out := make(map[string]map[string]string, len(body.Keys))
	for _, k := range body.Keys {
		out[k["kid"]] = k
	}
	return out
}
