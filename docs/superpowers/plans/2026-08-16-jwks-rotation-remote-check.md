# JWKS Rotation Remote Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `jwt.rotation_overlap` remote check to the developer self-service check suite that verifies a registered game correctly validates seat tokens signed under either of two simultaneously-active Lobby signing keys.

**Architecture:** Lobby's `auth.Signer` gains minimal, check-scoped support for a temporary second ed25519 keypair: it can be added to the live JWKS document and used to sign a token, then removed. A new check inside `RunJWTChecks` mints one token under the signer's primary key and one under a freshly added temporary key, POSTs both to the game's claim endpoint (same pattern as the existing `jwt.wrong_audience`/`jwt.wrong_issuer` checks), and passes only if both are accepted. This exercises the actual production `/.well-known/jwks.json` route (not a mock), since that's the real endpoint games fetch to verify tokens.

**Tech Stack:** Go 1.25, `github.com/golang-jwt/jwt/v5`, `crypto/ed25519`, `net/http/httptest` for tests.

## Global Constraints

- No GraphQL schema changes — `checkId` is already a plain `String!` (`backend/graph/schema/developer.graphqls:15`).
- No MCP changes — `mcp/joinquest-integration` passes check results through generically.
- `jwt.rotation_overlap` must NOT be added to `requiredPassChecks` (`backend/internal/store/developer.go:543`) — it is optional/non-gating, consistent with `provision.banlist`.
- No production key-rotation operations (no admin tooling, no way to actually rotate Lobby's live signing key). The temporary key exists only for the duration of a single check run.
- Follow existing code conventions in the touched files: `Result{CheckID, Status, Message}` struct, `StatusPass`/`StatusFail`/`StatusSkipped` constants, friendly human-readable fail messages matching the tone of neighboring checks.

---

### Task 1: Dual-key signing support in `auth.Signer`

**Files:**
- Modify: `backend/internal/auth/jwt.go`
- Modify: `backend/internal/auth/seat_token.go`
- Test: `backend/internal/auth/jwt_rotation_test.go` (new)

**Interfaces:**
- Produces: `func (s *Signer) AddRotationKey() (kid string, priv ed25519.PrivateKey, cleanup func(), err error)` — generates a temporary keypair, publishes its public half in `JWKSHandler`'s output, returns the kid, private key, and a cleanup func to remove it.
- Produces: `func (s *Signer) SignSeatTokenWithKey(kid string, priv ed25519.PrivateKey, userID uuid.UUID, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error)` — signs a seat token under an arbitrary (kid, private key) pair instead of the signer's primary key.
- `JWKSHandler()` (existing signature, unchanged) now serves the primary key plus any currently-registered rotation keys.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/auth/jwt_rotation_test.go`:

```go
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

func TestAddRotationKeyConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("JWT_DEV_KEY_FILE", filepath.Join(dir, "test.pem"))
	signer, err := LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	const n = 20
	cleanups := make(chan func(), n)
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, _, cleanup, err := signer.AddRotationKey()
			errs <- err
			cleanups <- cleanup
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("AddRotationKey: %v", err)
		}
		if cleanup := <-cleanups; cleanup != nil {
			cleanup()
		}
	}

	srv := httptest.NewServer(signer.JWKSHandler())
	t.Cleanup(srv.Close)
	jwks := fetchJWKSKeys(t, srv.URL)
	if len(jwks) != 1 {
		t.Fatalf("expected all rotation keys cleaned up, got %d keys", len(jwks))
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/auth/... -run 'RotationKey|SignSeatTokenWithKey' -v`
Expected: FAIL — compile error, `AddRotationKey` and `SignSeatTokenWithKey` undefined.

- [ ] **Step 3: Add the rotation key registry and JWKS serving to `jwt.go`**

In `backend/internal/auth/jwt.go`, add `"sync"` to the import block (after `"strings"`, before `"time"`):

```go
	"strings"
	"sync"
	"time"
```

Replace the `Signer` struct (current lines 27-33):

```go
// Signer creates and verifies session JWTs.
type Signer struct {
	kid        string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	publicX    string

	rotationMu   sync.RWMutex
	rotationKeys map[string]ed25519.PublicKey
}
```

Replace `PublicJWK` and `JWKSHandler` (current lines 123-141) with:

```go
// PublicJWK returns the public JWKS payload for this signer's primary key.
func (s *Signer) PublicJWK() map[string]string {
	return publicJWK(s.kid, s.publicX)
}

func publicJWK(kid, publicX string) map[string]string {
	return map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"use": "sig",
		"alg": "EdDSA",
		"kid": kid,
		"x":   publicX,
	}
}

// JWKSHandler serves the public JWKS document, including any temporary
// rotation keys added via AddRotationKey.
func (s *Signer) JWKSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		s.rotationMu.RLock()
		keys := make([]any, 0, 1+len(s.rotationKeys))
		keys = append(keys, s.PublicJWK())
		for kid, pub := range s.rotationKeys {
			keys = append(keys, publicJWK(kid, base64.RawURLEncoding.EncodeToString(pub)))
		}
		s.rotationMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}
}

// AddRotationKey generates a temporary ed25519 keypair and publishes its
// public half in this signer's JWKS document until the returned cleanup
// func is called. Integration checks use this to verify a game can validate
// tokens signed under more than one active key, as happens during a real
// signing-key rotation.
func (s *Signer) AddRotationKey() (kid string, priv ed25519.PrivateKey, cleanup func(), err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, nil, fmt.Errorf("auth: generate rotation key: %w", err)
	}
	kid = s.kid + "-rotate-" + uuid.NewString()

	s.rotationMu.Lock()
	if s.rotationKeys == nil {
		s.rotationKeys = make(map[string]ed25519.PublicKey)
	}
	s.rotationKeys[kid] = pub
	s.rotationMu.Unlock()

	cleanup = func() {
		s.rotationMu.Lock()
		delete(s.rotationKeys, kid)
		s.rotationMu.Unlock()
	}
	return kid, priv, cleanup, nil
}
```

- [ ] **Step 4: Add keyed signing to `seat_token.go`**

Replace the full contents of `backend/internal/auth/seat_token.go`:

```go
package auth

import (
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const defaultSeatTokenTTL = 2 * time.Hour

// SignSeatToken issues a short-lived JWT for launching into a third-party game.
// Claims: iss, aud, sub, jti, matchId, seatKey, name (optional), nbf, iat, exp.
func (s *Signer) SignSeatToken(userID uuid.UUID, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error) {
	return s.signSeatToken(s.kid, s.privateKey, userID, LobbyIssuer(), audience, externalMatchID, seatKey, displayName, ttl)
}

// SignSeatTokenWithIssuer signs a seat token using a custom iss claim (integration checks).
func (s *Signer) SignSeatTokenWithIssuer(userID uuid.UUID, issuer, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(issuer) == "" {
		return "", fmt.Errorf("auth: issuer is required")
	}
	return s.signSeatToken(s.kid, s.privateKey, userID, strings.TrimSpace(issuer), audience, externalMatchID, seatKey, displayName, ttl)
}

// SignSeatTokenWithKey signs a seat token under an alternate (kid, private
// key) pair instead of the signer's primary key. Used together with
// AddRotationKey to mint a token that only verifies against a temporary
// rotation key.
func (s *Signer) SignSeatTokenWithKey(kid string, priv ed25519.PrivateKey, userID uuid.UUID, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error) {
	return s.signSeatToken(kid, priv, userID, LobbyIssuer(), audience, externalMatchID, seatKey, displayName, ttl)
}

func (s *Signer) signSeatToken(kid string, priv ed25519.PrivateKey, userID uuid.UUID, issuer, audience, externalMatchID, seatKey, displayName string, ttl time.Duration) (string, error) {
	audience = normalizeAudience(audience)
	if audience == "" {
		return "", fmt.Errorf("auth: audience is required")
	}
	if externalMatchID == "" || seatKey == "" {
		return "", fmt.Errorf("auth: match id and seat key are required")
	}
	if ttl == 0 {
		ttl = defaultSeatTokenTTL
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":     issuer,
		"aud":     audience,
		"sub":     userID.String(),
		"jti":     uuid.NewString(),
		"matchId": externalMatchID,
		"seatKey": seatKey,
		"nbf":     now.Unix(),
		"iat":     now.Unix(),
		"exp":     now.Add(ttl).Unix(),
	}
	if displayName != "" {
		claims["name"] = displayName
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = kid
	return token.SignedString(priv)
}

func normalizeAudience(audience string) string {
	return strings.TrimRight(strings.TrimSpace(audience), "/")
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/auth/... -v`
Expected: PASS — all tests in the package, including the pre-existing `TestSeatTokenVerifiedViaJWKSHandler` and `TestSignAndVerifyUserToken` (confirms the refactor didn't break existing behavior).

Also run the race detector, since this task adds a mutex-guarded map:

Run: `cd backend && go test ./internal/auth/... -race -run Rotation -v`
Expected: PASS, no race warnings.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/auth/jwt.go backend/internal/auth/seat_token.go backend/internal/auth/jwt_rotation_test.go
git commit -m "$(cat <<'EOF'
Add temporary dual-key rotation support to auth.Signer

EOF
)"
```

---

### Task 2: `jwt.rotation_overlap` integration check

**Files:**
- Modify: `backend/internal/integrationchecks/provision.go`
- Test: `backend/internal/integrationchecks/jwt_rotation_test.go` (new)

**Interfaces:**
- Consumes: `signer.AddRotationKey()` and `signer.SignSeatTokenWithKey(...)` from Task 1.
- Produces: a `Result{CheckID: "jwt.rotation_overlap", ...}` appended into the slice returned by `RunJWTChecks`, and a corresponding `skipped("jwt.rotation_overlap", ...)` entry in both `jwtSkippedAll` and `jwtSkippedRest` for the existing early-return paths.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/integrationchecks/jwt_rotation_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/integrationchecks/... -run RotationOverlap -v`
Expected: FAIL — `resultStatus` finds no `jwt.rotation_overlap` entry in results (check doesn't exist yet), `t.Fatalf("check %q not found in results", checkID)`.

- [ ] **Step 3: Implement the check in `provision.go`**

In `backend/internal/integrationchecks/provision.go`, insert this block into `RunJWTChecks` immediately before the final `return results` (i.e. right after the existing `jwt.wrong_seat` / `else { skipped(...) }` block that currently ends the function):

```go
	if kid, rotPriv, cleanup, rotErr := signer.AddRotationKey(); rotErr == nil {
		func() {
			defer cleanup()

			primaryToken, primaryErr := signer.SignSeatToken(userID, audience, provisionMatchID, seatKey, "", time.Hour)
			rotationToken, rotationErr := signer.SignSeatTokenWithKey(kid, rotPriv, userID, audience, provisionMatchID, seatKey, "", time.Hour)
			if primaryErr != nil || rotationErr != nil {
				results = append(results, Result{
					CheckID: "jwt.rotation_overlap",
					Status:  StatusSkipped,
					Message: "Could not mint rotation test tokens.",
				})
				return
			}

			primaryStatus, _ := r.postClaim(ctx, claimURL, primaryToken)
			rotationStatus, _ := r.postClaim(ctx, claimURL, rotationToken)
			primaryOK := primaryStatus >= 200 && primaryStatus < 300
			rotationOK := rotationStatus >= 200 && rotationStatus < 300

			switch {
			case primaryOK && rotationOK:
				results = append(results, Result{
					CheckID: "jwt.rotation_overlap",
					Status:  StatusPass,
					Message: "Your game accepted valid tokens signed under two different active keys.",
				})
			case primaryOK && !rotationOK:
				results = append(results, Result{
					CheckID: "jwt.rotation_overlap",
					Status:  StatusFail,
					Message: fmt.Sprintf("Your game rejected a token signed with a newly added key while the old key was still active (HTTP %d) — verify tokens by matching the JWT's kid header against every key in the JWKS response, not just the first/cached one.", rotationStatus),
				})
			default:
				results = append(results, Result{
					CheckID: "jwt.rotation_overlap",
					Status:  StatusFail,
					Message: fmt.Sprintf("Your game rejected a validly signed token while two keys were active in JWKS (HTTP %d / %d).", primaryStatus, rotationStatus),
				})
			}
		}()
	} else {
		results = append(results, Result{
			CheckID: "jwt.rotation_overlap",
			Status:  StatusSkipped,
			Message: "Could not set up a temporary rotation key for this check.",
		})
	}

	return results
}
```

(This replaces the existing bare `return results\n}` at the end of `RunJWTChecks`.)

Then update `jwtSkippedAll` to add the new check ID, so early-return paths (no API URL, no signer, no provisioned match) report it consistently with the rest of the JWT group:

```go
func jwtSkippedAll(msg string) []Result {
	return []Result{
		skipped("jwt.jwks", msg),
		skipped("jwt.claim_happy_path", msg),
		skipped("jwt.wrong_audience", msg),
		skipped("jwt.unknown_match", msg),
		skipped("jwt.wrong_issuer", msg),
		skipped("jwt.expired", msg),
		skipped("jwt.invalid_token", msg),
		skipped("jwt.wrong_seat", msg),
		skipped("jwt.rotation_overlap", msg),
	}
}
```

And `jwtSkippedRest` (used when the happy-path token fails to mint):

```go
func jwtSkippedRest(msg string) []Result {
	return []Result{
		skipped("jwt.wrong_audience", msg),
		skipped("jwt.unknown_match", msg),
		skipped("jwt.wrong_issuer", msg),
		skipped("jwt.expired", msg),
		skipped("jwt.invalid_token", msg),
		skipped("jwt.wrong_seat", msg),
		skipped("jwt.rotation_overlap", msg),
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/integrationchecks/... -v`
Expected: PASS — both new tests plus the pre-existing `TestLaunchURLsContainJWT`.

- [ ] **Step 5: Run the full backend test suite**

Run: `cd backend && go build ./... && go test ./...`
Expected: PASS, no build errors, no regressions elsewhere (confirms the `seat_token.go` refactor from Task 1 didn't break any other caller of `SignSeatToken`/`SignSeatTokenWithIssuer`).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/integrationchecks/provision.go backend/internal/integrationchecks/jwt_rotation_test.go
git commit -m "$(cat <<'EOF'
Add jwt.rotation_overlap integration check (JQ-7)

EOF
)"
```

---

### Task 3: Update developer self-service documentation

**Files:**
- Modify: `docs/developer-self-service.md`

**Interfaces:**
- None (documentation only).

- [ ] **Step 1: Move the check out of "Still open" and off the top-level Next line**

In `docs/developer-self-service.md`, change line 4 from:

```
**Next:** public release review polish, scheduled re-checks, JWKS rotation remote check
```

to:

```
**Next:** public release review polish, scheduled re-checks
```

- [ ] **Step 2: Add a row to the JWT verification checklist table**

In the "#### 3. JWT verification" table, after the "Wrong seat" row, add:

```
| Rotation overlap | Token signed under a temporary second key is accepted while both keys are active in JWKS (optional, non-gating) | "Your game rejected a token signed with a newly added key while the old key was still active — verify tokens by matching the JWT's `kid` header against every key in the JWKS response, not just the first/cached one." |
```

- [ ] **Step 3: Remove the now-shipped follow-up bullet**

In the "Phase B — Go public + agents" section's "Still open (Phase B follow-ups)" list, remove this line:

```
- Remote `jwt.rotation_overlap` check (needs Lobby dual-key JWKS during rotation)
```

- [ ] **Step 4: Commit**

```bash
git add docs/developer-self-service.md
git commit -m "$(cat <<'EOF'
Document jwt.rotation_overlap check in developer self-service guide

EOF
)"
```
