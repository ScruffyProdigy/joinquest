package integrationchecks

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/store"
)

func TestRunJWTChecksRotationOverlapPassesWhenGameChecksKid(t *testing.T) {
	signer, jwksSrv := testSignerWithJWKS(t)
	primaryKid := signer.PublicJWK()["kid"]

	gameSrv := newMockGameClaimServer(t, jwksSrv.URL, primaryKid, false)
	t.Cleanup(gameSrv.Close)

	game := testGame(gameSrv.URL)
	runner := &Runner{}
	results := runner.RunJWTChecks(context.Background(), game, signer, "match-1", "seat-a", "")

	if got := resultStatus(t, results, "jwt.rotation_overlap"); got != StatusPass {
		t.Fatalf("expected jwt.rotation_overlap to pass, got %s", got)
	}
}

func TestRunJWTChecksRotationOverlapFailsWhenGameIgnoresKid(t *testing.T) {
	signer, jwksSrv := testSignerWithJWKS(t)
	primaryKid := signer.PublicJWK()["kid"]

	gameSrv := newMockGameClaimServer(t, jwksSrv.URL, primaryKid, true)
	t.Cleanup(gameSrv.Close)

	game := testGame(gameSrv.URL)
	runner := &Runner{}
	results := runner.RunJWTChecks(context.Background(), game, signer, "match-1", "seat-a", "")

	if got := resultStatus(t, results, "jwt.rotation_overlap"); got != StatusFail {
		t.Fatalf("expected jwt.rotation_overlap to fail, got %s", got)
	}
}

func testSignerWithJWKS(t *testing.T) (*auth.Signer, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("JWT_DEV_KEY_FILE", filepath.Join(dir, "test.pem"))
	signer, err := auth.LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	jwksSrv := httptest.NewServer(signer.JWKSHandler())
	t.Cleanup(jwksSrv.Close)
	t.Setenv("LOBBY_ISSUER_URL", jwksSrv.URL)

	return signer, jwksSrv
}

func testGame(apiBaseURL string) *store.Game {
	return &store.Game{APIBaseURL: &apiBaseURL}
}

func resultStatus(t *testing.T, results []Result, checkID string) string {
	t.Helper()
	for _, r := range results {
		if r.CheckID == checkID {
			return r.Status
		}
	}
	t.Fatalf("check %q not found in results", checkID)
	return ""
}

// newMockGameClaimServer simulates a registered game's claim endpoint. When
// ignoreKid is false it verifies each token by looking up its kid header in
// the fetched JWKS (correct behavior). When true it always verifies against
// primaryKid regardless of the token's actual kid — simulating a game that
// cached a single key and never re-checks by kid, which is the real bug
// jwt.rotation_overlap exists to catch.
func newMockGameClaimServer(t *testing.T, jwksURL, primaryKid string, ignoreKid bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		keys := fetchJWKSKeys(t, jwksURL)

		_, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
			lookupKid := primaryKid
			if !ignoreKid {
				if kid, ok := token.Header["kid"].(string); ok {
					lookupKid = kid
				}
			}
			k, ok := keys[lookupKid]
			if !ok {
				return nil, fmt.Errorf("unknown kid %q", lookupKid)
			}
			raw, err := base64.RawURLEncoding.DecodeString(k["x"])
			if err != nil {
				return nil, err
			}
			return ed25519.PublicKey(raw), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
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
