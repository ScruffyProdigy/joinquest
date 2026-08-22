# JWKS rotation remote check — design

**Ticket:** [JQ-7](https://linear.app/joinquest/issue/JQ-7/jwks-rotation-remote-check)
**Related:** [developer-self-service.md](../../developer-self-service.md) (Phase B follow-ups: "Remote `jwt.rotation_overlap` check (needs Lobby dual-key JWKS during rotation)")

## Context

Lobby is the JWT issuer for seat tokens; registered games are the verifiers, fetching Lobby's `/.well-known/jwks.json` to validate tokens. The existing check suite (`backend/internal/integrationchecks/`) already covers JWT verification edge cases (wrong audience, wrong issuer, expired, malformed, wrong seat), but nothing tests whether a game's JWKS-consuming logic can handle **more than one active signing key at once** — which is a prerequisite for Lobby ever rotating its signing key without breaking already-integrated games mid-rotation.

Today `auth.Signer` (`backend/internal/auth/jwt.go`) holds exactly one ed25519 keypair. There is no dual-key or rotation support anywhere in the codebase.

## Scope decision

This ticket adds:

1. Minimal, **check-scoped** dual-key support in `auth.Signer` — enough to mint tokens under two keys and serve both in JWKS for the duration of a single check run.
2. A new `jwt.rotation_overlap` check that verifies a game accepts tokens signed under either key when both are present in the JWKS document.

This ticket explicitly does **not** build:

- Production key-rotation operations (no way to actually rotate Lobby's live signing key, no admin tooling, no scheduled/automatic rotation).
- Time-based cache-refresh testing (verifying a game re-polls JWKS on some interval). A single remote check run cannot deterministically observe a game's polling schedule. What it *can* verify is that the game's verification logic does `kid`-based key lookup against whatever JWKS document it currently has — which is the actual requirement for rotation to work without downtime once Lobby does rotate keys in production.

Follow-up work (building real production rotation) is out of scope and would be its own ticket if/when Lobby needs to actually rotate its signing key operationally.

## Release gating

`jwt.rotation_overlap` is **optional/non-gating** — it is not added to `requiredPassChecks` (`backend/internal/store/developer.go`). It runs automatically as part of the JWT check group and reports pass/fail like every other check, but does not block `RequestPublicRelease`. This matches the doc's framing of it as a Phase B follow-up rather than one of the 19 required checks, and avoids retroactively failing games that are already public the moment this ships.

## Design

### 1. Dual-key support in `auth.Signer`

- `backend/internal/auth/jwt.go`: add an optional secondary keypair a `Signer` can hold alongside its primary — e.g. a `rotationKey *signingKey` field (kid + ed25519 priv/pub), set via a constructor option or a method such as `Signer.WithRotationKey(kid string, priv ed25519.PrivateKey, pub ed25519.PublicKey)`.
- `PublicJWK()` / `JWKSHandler()` (`jwt.go:124,136`) emit both keys in the JWKS `keys` array when a rotation key is set, and just the one key otherwise — existing single-key behavior is the default and is unaffected for normal signer instances (route at `backend/server.go:90`).
- A new signing method mirroring `SignSeatTokenWithIssuer` (`seat_token.go:21`) that signs specifically under the rotation key, so the check can mint a token per key without touching the normal seat-token issuance path used by real matches.
- The check constructs its own **throwaway `Signer` instance** (ephemeral rotation keypair, generated fresh per check run) rather than mutating the shared long-lived signer used for real traffic. This avoids any risk of a second key leaking into the production JWKS during concurrent check runs for other games, and needs no cleanup/teardown step.

### 2. New check: `jwt.rotation_overlap`

In `backend/internal/integrationchecks/provision.go`, alongside `RunJWTChecks` (`:240`):

- Build a throwaway signer with a fresh rotation keypair alongside the real primary key material used elsewhere in the JWT check group.
- Mint token A under the primary key, token B under the rotation key, both otherwise valid (correct audience, issuer, seat, not expired) — same shape as the happy-path token used in `jwt.claim_happy_path`.
- POST both via the existing `postClaim` helper (`:432`) to the game's `/api/v1/matches/{id}/claim`.
- Pass only if **both** return 200. If the game only accepts whichever key it cached first, or only checks "the" key rather than looking up by `kid`, one of the two will fail with 401/403 — exactly the failure mode this check exists to catch.
- Failure message follows the existing convention, e.g.: "Your game only accepted one of two valid signing keys — verify JWKS-based tokens by matching the token's `kid` header against all keys in the JWKS response, not just the first/cached one."

### 3. Wiring

`backend/graph/developer_checks.go:18` (`persistGameChecks`) calls the new check function alongside the existing `RunJWTChecks` call, same pattern as the other check groups. No GraphQL schema change needed — `checkId` is a plain `String!` (`backend/graph/schema/developer.graphqls:15`), and MCP (`mcp/joinquest-integration/src/tools.js`) passes check results through generically.

### 4. Documentation

Update `docs/developer-self-service.md`:

- Move "Remote `jwt.rotation_overlap` check" out of "Still open (Phase B follow-ups)" once shipped.
- Add a row to the "3. JWT verification" checklist table describing the check and its fail message, consistent with the existing rows.

## Testing

- `backend/internal/auth`: table-driven unit tests for dual-key JWKS serving and rotation-key signing, alongside the existing `jwks_roundtrip_test.go` pattern.
- `backend/internal/integrationchecks`: test for `jwt.rotation_overlap` using `httptest.NewServer` as a mock game claim endpoint:
  - Mock accepts both keys → check passes.
  - Mock only accepts the primary key (simulating the real bug this check catches) → check fails with the expected message.
- No changes needed to GraphQL or MCP test surfaces since no schema/tool contract changes.

## Open questions

None — scope, gating, and mechanism were confirmed during brainstorming.
