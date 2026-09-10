# Lobby ↔ Game Protocol — Hand-off & Rationale

A brief for **game developers** integrating with JoinQuest and for contributors implementing the **Lobby** side.

**Product context:** Most teams should not rebuild cross-origin auth, push provisioning, and banlist recovery. JoinQuest sends you players and a **final seat map**; you focus on the game client and server. Read [`vision.md`](./vision.md) for the full “build the game, we handle the lobby” story.

This document covers the wire contract, **why each part is shaped the way it is**, and how the same contract scales from 1v1 to larger modes.

For step-by-step request/response bodies, see the **Provision payload** section below and the `demo-game-rps` reference implementation. This doc is the *reasoning* companion — read it so contract decisions don't look arbitrary.

---

## TL;DR of the model

1. **Discovery** (REST): Lobby reads the game's seat manifest.
2. **Provision / push** (REST, server-to-server): Lobby creates the match on the
   game *before* any player arrives. The game can reject the roster here.
3. **Link-out** (browser): Lobby sends each player to a **game-minted URL** with a signed JWT attached (`token` query param). Games must return launch URL bases on provision — there is no catalog fallback.
4. **Claim + play** (browser → WebSocket): the player claims their reserved seat
   with the token, then plays over a WebSocket.

Lobby decides **who** sits in **which** seat. The game defines **what the seats
are** and whether it will **accept** the roster.

---

## Why the contract looks like this

These are the decisions that shaped the current contract, and the reasoning the
Lobby agent should internalize. Several of these *changed* during design — the
rationale explains what we moved away from and why.

### 1. Push provisioning ("option 2"), not link-only
**What we do:** Lobby calls `POST /api/v1/matches` server-to-server to create the
match *before* redirecting players.

**Why:** The obvious alternative is "just link the players over and let the game
lazily create the match on first arrival." We rejected that because it gives Lobby
**no synchronous point to be told 'no.'** With a push, the game gets to inspect
the whole roster up front and respond before any user is mid-flow. Concretely,
this is what enables the **banlist handshake** (below). It also keeps a single
source of truth: the pushed match is authoritative, so two players can't race to
create conflicting match rooms.

**Lobby impact:** provisioning is a deliberate step in your matchmaking pipeline,
not a side effect of a redirect.

### 2. The push can be rejected (banlist handshake)
**What we do:** the push returns `403 { bannedLobbyUserIds: [...] }` if the game
refuses a player.

**Why:** the game — not Lobby — owns *acceptance*. A third-party game may ban a
user for cheating/abuse that Lobby has no knowledge of. Returning the offending
ids (not just a generic error) lets Lobby **correct automatically**: drop/replace
the player and re-push, rather than dumping a dead-end error on the user. This is
the entire reason push happens before the link-out, not after.

**Lobby impact:** treat `403` as "re-matchmake," not "fatal."

### 3. The match definition is pushed, not carried in the token
**What we do:** the JWT contains only identity + seat (`sub`, `matchId`,
`seatKey`, `name`). The roster/mode live in the pushed match.

**Why:** an earlier design put the whole roster inside the token. That's worse:
tokens then become large, hard to revoke, and the "truth" about a match would be
duplicated in N tokens that could disagree. By making the **pushed match the
single source of truth** and the **token just a proof of identity**, a token
claim against a match that was never provisioned correctly fails with `404` —
which is the property that keeps Lobby in control of provisioning. Tokens stay
small and only answer "is this really user X, and which seat are they allowed to
take?"

**Lobby impact:** mint minimal tokens; don't try to encode match state in them.

### 4. Identity travels as a signed JWT in the URL, verified via JWKS
**What we do:** Lobby signs a short-lived JWT and the game verifies it against
`LOBBY_JWKS_URL`.

**Why:** the game client (`:5174`) and Lobby (`:5173`/`:8080`) are **different
origins**, so Lobby's session cookie does not cross over. We needed a way to prove
identity across that boundary without the game ever holding Lobby credentials.
Asymmetric signing + JWKS does exactly this: the game only ever needs Lobby's
**public** keys, which it can fetch and cache. No shared secrets, and key rotation
is handled by publishing new keys at the JWKS endpoint — no coordinated deploy
with each game.

**Lobby impact:** you must expose `/.well-known/jwks.json` and sign per-user seat
tokens. The seat token is what stops a user from grabbing `white` or stealing a
MOBA slot — **the token, not the URL query param, is authoritative.**

### 5. Seats are a published *manifest*, not a hardcoded player count
**What we do:** the game publishes `GET /api/v1/game-modes` with `minPlayers`,
`maxPlayers`, and **`seatTemplate`** (a layout tree Lobby expands to leaf `seatKey`s).
Flat `seats[]` is rejected. Optional `team` / `role` on provision seats may follow
from the template in later phases.

**Why:** the integration must not be RPS-specific. The original ask generalized to
"works for >2 players, with specific roles/teams (chess white/black, MOBA teams)."
By making seats data the game declares (rather than a number Lobby assumes), the
**same provisioning contract describes a 1v1, a chess match, or a 5v5** — Lobby
just maps matched players onto the `seatKey`s the game advertises. No protocol
change to onboard a new game shape.

**Lobby impact:** drive your matchmaker from the manifest. Never bake in "2
players."

### 6. Gameplay is a WebSocket, not polling
**What we do:** after claiming, the client opens `GET /api/v1/ws` and subscribes.

**Why:** this repo is meant as a **template** other game authors copy. Polling is
a poor default for real-time games (latency, wasted load), and most game authors
will want push/subscription transport. WebSockets are the sensible default even
though RPS itself only needs server→client pushes — games in general benefit from
a duplex channel, so the template ships one.

**Lobby impact:** essentially none — this is internal to the game's client↔server
transport. **Lobby's contract stays REST** (push + signed link-out). Don't build
Lobby against the WS; it's the game's concern.

### 7. `externalMatchId` is the join key everywhere
**What we do:** Lobby's `externalMatchId` is what the game stores, what the token's
`matchId` references, what the claim URL uses as `:ref`, and what WS subscribes to.

**Why:** one stable, Lobby-owned identifier threaded through every step means
Lobby never has to learn a game-internal id, and idempotent retries "just work"
(re-pushing the same `externalMatchId` returns the same match). It also becomes the
natural **routing/shard key** when the game scales out (see below).

**Lobby impact:** generate a stable, unique `externalMatchId` per match and reuse
it for the token and the link.

### 8. A player may re-claim their own seat; nobody else may take it
**What we do:** a claim against an already-claimed seat succeeds when the token's
`sub` matches the player already assigned to that seat, and still returns `409` for
any other `sub`.

**Why:** the original rule was a flat `409` on any second claim, which is correct
against seat theft and wrong about the far more common case — the same player coming
back. Players close tabs, lose wifi, and take phone calls mid-match; for a casual
audience that is routine, not an edge case, and a player locked out of their own seat
ruins the match for everyone still in it. Comparing `sub` separates the two cases
exactly: "this seat is taken" is only an error when it is taken **by somebody else**.

Nothing about the seat changes on a re-claim. It is the same player, the same seat,
and the same match — so re-claiming must be **idempotent on match state**: no reset,
no second join event, no re-deal, no turn advanced, no rematch of an already-started
round. Treat it as "resend me the current state," not "join."

**Lobby impact:** Lobby re-mints a seat token for a live session on demand
(`rejoinActiveMatch`), keeping `sub`, `matchId` and `seatKey` and issuing a fresh
`jti`/`exp`. Re-mint is refused once the match is over or that player has reported
finished, so a re-claim that arrives after the match ended cannot be holding a token
Lobby issued for it. Re-mint tokens are minted on demand and are deliberately
short-lived (minutes, not hours) — long enough to travel, too short to pass around.

**Game impact:** compare `sub` before returning `409`, and make the claim handler
safe to run twice for the same player. See *Reconnecting a player* below.

---

## Protocol reference (condensed)

Base URLs are per-game and per-environment. Store **`apiBaseUrl`** (server-to-server origin) on each catalog row. Browser launch URLs come from the game’s provision response, not the catalog.

| Step | Call | Notes |
|------|------|-------|
| Health | `GET {apiBaseUrl}/healthz` → `ok` | gate listing on this |
| Status | `GET {apiBaseUrl}/api/v1/status` → `{game,version,appEnv,standalone,launchUrlsOnProvision?}` | capability gating; `launchUrlsOnProvision: true` when game mints URLs |
| Modes | `GET {apiBaseUrl}/api/v1/game-modes` | `seatTemplate` per mode (expanded by Lobby) |
| Provision | `POST {apiBaseUrl}/api/v1/matches` body `{ lobbyId, lobby: { returnUrl, graphqlUrl, serviceToken? }, assignment: { ... } }` | S2S; `Authorization: Bearer` must match `serviceToken` when present; idempotent on `externalMatchId`. **Response:** `{ launchUrls?: { [lobbyUserId]: string }, launchUrlTemplate?: string, … }` — URL bases without JWT |
| Link | game URL base from provision + Lobby attaches `token=<jwt>` | optional `seat`, `lobby_user` may already be in game URL |
| Claim | `POST {apiBaseUrl}/api/v1/matches/{externalMatchId}/claim` + `Authorization: Bearer <jwt>` | game uses the token's `seatKey` |
| Play | WebSocket `GET /api/v1/ws` (or REST `POST /matches/:ref/move`) | game-internal transport |

**Lobby block:** `returnUrl` is the **return hub** (`/return` on the Lobby origin). Games should send players to `{returnUrl}?match={externalMatchId}` after a match. See [player-return-routing.md](./player-return-routing.md). `graphqlUrl` is the Lobby GraphQL endpoint; `serviceToken` is a per-game HMAC token (`v1.{gameId}.{sig}`) when `LOBBY_GAME_TOKEN_PEPPER` is set — games store it on the match for `player` lookup and lifecycle callbacks. Omit in local dev when no pepper is configured.

**Assignment seat shape:** `{ seatKey, lobbyUserId, team?, role? }`. The `seatKey` is the
expanded name from the game’s `seatTemplate` (derivation rules in
[`seat-templates-and-matchmaking.md`](./seat-templates-and-matchmaking.md#seat-key-expansion)).
Resolve display names and **avatar URLs** via `player(id)` at `lobby.graphqlUrl` (`displayName`, `avatarUrl`). See [spirit-animal-avatars.md](./spirit-animal-avatars.md).

**Token claims:** `iss`=`LobbyIssuer()`, `aud`=game `api_base_url`, `sub`=lobby user id,
`jti`=unique token id, `matchId`=`externalMatchId`, **`seatKey`** (same string as provision
and session assignment), optional `name`, `nbf`/`iat`/`exp`. Games should reject tokens
whose `iss` does not match provision `lobbyId`, whose `aud` does not match their API base
URL, or whose `seatKey` does not match the seat Lobby assigned that player.

**Status codes:** `200/201` ok · `400` bad input/illegal move · `401` bad/missing
token · `403` **banned** (`bannedLobbyUserIds[]`) **or** seat-reservation violation
· `404` unknown/unprovisioned match · `409` conflict (seat taken **by a different
`sub`** / duplicate move). A claim on a seat already held by the **same** `sub` is a
re-claim and returns `200`, not `409` — see *Reconnecting a player*.

---

## Reconnecting a player

A player who closes the tab, drops their connection, or takes a phone call mid-match
needs a defined way back into the seat they still hold. There are **two independent
paths back**, and they are independent on purpose — each one works when the other has
failed, so no single lost cookie or closed tab strands a player.

### Path 1 — game-origin recovery (primary, no Lobby round trip)

**On a successful claim, the game binds browser → seat on its own origin.** Set a
cookie (or equivalent local record) on the game's own domain recording which
`seatKey` of which `externalMatchId` this browser holds, alongside the verified
`sub`. On a later request to the game with no `?token=`, look that binding up and
resume the player.

This is the path that covers a refresh, the back button, and a tab crash: it involves
no Lobby request at all, so it keeps working even if Lobby is slow, unreachable, or
the player's Lobby session is gone. It is also the fastest — the player is back in
the match without a navigation through JoinQuest.

The binding must be **checked against the match, not trusted on its own**: it names a
seat, and the match is still the source of truth for whether that seat is live.

### Path 2 — lobby-origin recovery (fallback, the Rejoin button)

A player who comes back through JoinQuest instead — bookmark, home screen, a link a
friend sent — sees their live match on the Lobby with a **Rejoin** action. Rejoin
mints a fresh seat token for the seat they already hold and navigates them to the
game's launch URL, where the normal claim runs and lands on the re-claim rule above.

This is the path that covers a player whose game-origin binding is gone: different
browser profile, cleared game-site data, or a game they reached from a new device.

### Why `SameSite=Lax` shapes how games return players

Lobby's session cookie is `SameSite=Lax`. That means it **is** sent on a top-level
navigation back to the Lobby origin — a link click, a `window.location` assignment, a
form GET — and is **not** sent on a cross-site POST or a background `fetch`/XHR.

So: **return players by navigation.** A game that sends the browser to
`{returnUrl}?match={externalMatchId}` gets an authenticated player on the other side.
A game that tries to hand off by POSTing to Lobby, or by fetching a Lobby endpoint
from its own page, gets an anonymous request and a player who looks logged out.

### Session lifetime, and the one case that is not recoverable

Lobby's session cookie is persistent with a 7-day `MaxAge`, and `SESSION_TTL` matches
it, so a guest's identity survives a browser restart. Both were **absolute from
creation** with no renewal, which meant an active guest could expire out of their own
identity mid-play; Lobby now **slides** the session, re-issuing the cookie on any
authenticated request made past roughly half the token's life. A player who keeps
playing therefore never ages out.

What remains unrecoverable is a guest whose cookie is genuinely gone: an incognito
window that was closed, cookies cleared, or a different device entirely. There is no
credential to re-authenticate a guest with, so that identity — and that player's
history with every integrated game — is gone.

**This is accepted, not engineered around.** Recovering it would mean giving guests a
credential, which is an account by another name. It is instead the natural trigger
point for the sign-up upsell: the moment a player has something worth not losing is
the moment to offer them a way to keep it.

### What a game should do about a disconnect it can see

Games differ, so this is guidance rather than contract, but behaviour should be
consistent enough across titles that a player learns one set of expectations:

- **Hold the seat.** A disconnected player's seat stays theirs. Do not forfeit them,
  do not fill the seat, and do not end the match on the first dropped socket.
- **Say so.** Show the remaining players that someone is disconnected and that the
  game is waiting — "Waiting for Alex to reconnect…" — rather than leaving the match
  silently stalled.
- **Keep the match moving where the design allows.** In a real-time game, continue
  and let the returning player catch up. In a turn-based game, it is reasonable to
  hold on their turn with a visible timer rather than freeze indefinitely.
- **Resync from a full snapshot.** When the player re-claims, send complete
  authoritative state, not the deltas they missed. They should never have to guess.
- **Have an end to the grace period.** Decide what happens if they never come back —
  a timeout that forfeits, or an abandon vote — and tell the other players what it is.
  An indefinite wait is worse for the people still there than a decided outcome.

Filling an empty seat with a bot or a replacement player is deliberately **not** part
of this; it is a separate concern.

---

## Scaling to larger servers (and why)

Bake these into Lobby's model now so bigger games and a multi-instance game fleet
don't force a redesign later.

### Treat each game as a registered backend, not a URL
**Do:** store `slug`, **`apiBaseUrl`**, and a cached copy of the game's
`/api/v1/game-modes` per catalog entry.

**Why:** push goes to `apiBaseUrl`, players go to URLs the game returns on provision, and matchmaking needs
the manifest. Modeling a game as `{apiBaseUrl, modes}` is what lets you
add a second/third game with zero protocol change — the only new data is another
row.

### Keep matchmaking seat-template-driven
**Do:** read `minPlayers`/`maxPlayers`/`teams`/`seats` from the manifest and map
matched players onto `seatKey`s in the push.

**Why:** a 5v5 is just `seats:[{seatKey:"radiant-mid",team:"radiant",role:"mid"},…]`
— the same `assignment` shape with more rows. If any part of Lobby special-cases
"2 players," larger games break. The contract was generalized precisely to avoid
that.

### `externalMatchId` is the shard key for a game fleet
**Do:** always send a stable `externalMatchId`; don't assume which game instance
serves it.

**Why:** the demo keeps match state in Postgres but fans WebSocket updates out via
an **in-process** hub. That's correct for one instance, but across a horizontally
scaled game fleet, an in-process hub won't reach subscribers on other instances.
The game team must either (a) pin all traffic for a given `externalMatchId` to one
instance (sticky routing by match id) or (b) back the hub with shared pub/sub
(e.g. Redis). **This is a game-side concern**, but Lobby enables it for free by
making the match id the routing key and not caring about instance identity. Flag
it to the game team so they don't ship the in-process hub to production unchanged.

### Authenticate the push before production
**Do:** agree on a Lobby→game service credential (shared secret header, signed
request, or mTLS) and send it on `POST /api/v1/matches`. The server already
supports requiring it (`REQUIRE_LOBBY_AUTH=true`).

**Why:** in the local demo the push is unauthenticated, which is fine for a laptop
but means **anyone could provision matches** in a real deployment. Player tokens
are JWKS-verified, but the *server-to-server* push needs its own auth. This is the
top hardening item for "larger servers."

### Gate on `version` / `healthz`
**Do:** treat `/healthz` failures as "temporarily unlistable," and use
`/api/v1/status` `version` to detect protocol drift.

**Why:** with many independently deployed games, you need a cheap way to avoid
sending players to a down or incompatible game. This leaves room for explicit
capability negotiation later without breaking older games.

---

## Things still open / not frozen

- **JWT claim names** (`matchId`/`seatKey`/`name`) — the game also accepts
  `match_id`/`seat_key`/`displayName`, but lock these down together in "Lobby PR 2."
- **Push authentication** — unauthenticated in the demo; must be added before
  non-local use (see above).
- **Persistent banlist** — currently `BANNED_LOBBY_USERS` env list; a table is
  future work. Doesn't change the wire contract.
- **JWKS rotation/cache tuning** — `jose`'s `createRemoteJWKSet` caches keys;
  production rotation policy is future work.

---

## Lobby implementer checklist

1. Seed a catalog row: `slug`, `name`, **`apiBaseUrl`**, status.
2. Cache the game's `/api/v1/game-modes`; drive matchmaking from it (no hardcoded
   counts).
3. At match start: `POST /api/v1/matches` with the assignment. Handle `403` by
   re-matchmaking (don't surface it to the user).
4. Expose `/.well-known/jwks.json`; mint a per-user seat JWT (`sub`, `matchId`,
   `seatKey`, `name`).
5. On successful provision, read `launchUrls` (or expand `launchUrlTemplate`); attach JWT to each base URL. Persist URL bases on session participants for refresh.
6. Add the **Play** button / intent banner that opens the final link from step 5.
   The banner's action for a live match mints on click (`rejoinActiveMatch`), because
   a rejoin token is short-lived and a URL fetched when the banner rendered would be
   stale by the time a returning player used it.
7. Before production: authenticate the push and confirm the game's WS fan-out is
   fleet-safe. Reference games set `GAME_PLAY_URL` to their browser origin for minted links.
