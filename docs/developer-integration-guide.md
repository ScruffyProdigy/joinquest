# JoinQuest developer integration guide

> **Canonical source:** Edit here, then run `./scripts/sync-developer-docs.sh` to update the backend embed (`backend/internal/developer/integration_guide.md`) served via GraphQL/MCP.

Single source of truth for humans, dashboard copy, and MCP agents (`joinquest_integration_get_integration_guide`).

**Agent workflow:** [developer-agent-playbook.md](./developer-agent-playbook.md) — end-to-end phases for vibe coding (`joinquest_integration_get_agent_playbook`). Bundled as an [Agent Skill](../.agents/skills/joinquest-integration/SKILL.md) at `.agents/skills/joinquest-integration/`.

**Product context:** [developer-self-service.md](./developer-self-service.md) · Wire contract: [lobby-protocol-handoff.md](./lobby-protocol-handoff.md) · Seat layouts: [seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md)

---

## 0. Discover your game (agents + developers)

Before writing API code or catalog copy, make sure you (or your agent) understand what you're building. Start open-ended, listen, then ask clarifying questions only for what's still unclear. Draft metadata and manifest suggestions for developer approval.

### Start open-ended

Invite the developer to describe the game in their own words — no checklist yet:

> Tell me about the game you're thinking of — as much or as little as you have. What's the idea, how do people play together, who it's for, anything you're excited about or unsure about.

### Clarify only what's missing

Read their description and identify gaps. Ask follow-ups conversationally — one or two at a time. If they already answered something, do not re-ask it.

**Confirm what you think you know:** If you can infer an answer but aren't fully sure, check it with the developer instead of guessing or asking from scratch. For example: "It sounds like this is mostly a 2-player game — would you say that's fair?"

| Topic | Why it matters | Only ask if unclear |
|-------|----------------|---------------------|
| Player count | seatTemplate / game-modes | min/max, fixed or variable |
| Structure | seatTemplate | duel, free-for-all, teams, or roles |
| Genre | `genre` | what the game is *about*: action, strategy, deduction, words, drawing, puzzle |
| Social mode | `socialMode` per mode | free-for-all, 1v1, teams, hidden roles, or co-op — ask per mode, they often differ |
| Session length | `typicalMinutes` per mode | roughly how long one round runs — ask per mode, a Duel is usually shorter than an Arena |
| Difficulty | `difficulty` | how much a new player must know before their first round is fun |
| Vibe / audience | catalog voice | brainy, chaotic, tactical, etc. — copy, not a field |
| API URL | registration | public HTTPS hosting plan (not localhost) |

### From answers → manifest + axes

| If the game is… | Start with | Mode `socialMode` |
|-----------------|------------|-------------------|
| Two players head-to-head | `{ "count": 2 }` duel mode | `1v1` |
| Team vs team | `Team: { count: 2, Seat: { count: N } }` | `teams` |
| Co-op in one group | `{ "count": N }` or single team template | `co-op` |
| Everyone for themselves | `{ "count": N }` | `free-for-all` |
| Secret allegiances | Role buckets in `seatTemplate` | `hidden-roles` |

`socialMode` is per **mode**, not per game: a game with an Arena mode and a Duel
mode declares `free-for-all` on one and `1v1` on the other. `genre` and
`difficulty` are per **game** and are set with `updateMyGameMetadata`.

See the [seat template cookbook](./seat-templates-and-matchmaking.md#cookbook) for JSON examples.

Agents: after discovery, call `updateMyGameMetadata` with drafted copy (developer confirms) and implement the suggested `seatTemplate` on the game API.

---

## 1. Catalog voice (JoinQuest tone)

Lobby copy is **warm, plain, and player-first** — not enterprise, not hype.

**Do**

- Use short sentences. Card blurbs ~120 characters.
- Say “look for a group” / “play together”, not “enter matchmaking queue”.
- Describe what players *do*, not tech stack.
- Match the friendly tone of JoinQuest: *“Find your group. Play together.”*

**Don't**

- Stack buzzwords (“next-gen social competitive ecosystem”).
- Mention JWT, GraphQL, or provision in player-facing copy.
- Use ALL CAPS or excessive exclamation marks.

**Examples**

| Weak | Better (short description) |
|------|------------------------------|
| “Real-time multiplayer word game leveraging WebSocket architecture” | “Race friends to find words on a shared grid.” |
| “The ultimate 1v1 RPS experience!!!” | “Classic rock paper scissors — best of three, no accounts on your side.” |

**Field guide**

| Field | Audience | Length / notes |
|-------|----------|----------------|
| `shortDescription` | Catalog card | 1–2 sentences, ~120 chars |
| `longDescription` | Detail page | 2–4 short paragraphs; what it is, why it's fun |
| `howToPlay` | Detail page | 3–6 bullet steps; first-time player |
| `genre` | Catalog card label + browse filter | exactly one id from [genre](#catalog-axes) |
| `difficulty` | Catalog chip | optional; one id from [difficulty](#catalog-axes) |

---

## 2. Catalog axes

The catalog describes a game on three axes, each answering one question. They
replaced the flat `tags` list, which mixed all three and so could not be relied
on to say anything in particular.

### `genre` — what the game is about (per game, exactly one)

| ID | Label | Use when |
|----|-------|----------|
| `action` | Action | Reflexes, timing, real-time pressure |
| `strategy` | Strategy | Planning, tactics, long-run decisions |
| `deduction` | Deduction | Hidden information, reading other players |
| `words-trivia` | Words & Trivia | Language, knowledge, recall |
| `drawing-creative` | Drawing & Creative | Making something others judge |
| `puzzle` | Puzzle | Solving a defined problem |

One per game, because the card shows one label and a game claiming two claims
neither. Pick the one a player would use to find you.

### `difficulty` — the floor to enjoy it (per game, optional)

| ID | Label | Use when |
|----|-------|----------|
| `casual` | Casual | Low pressure, playable without instructions |
| `involved` | Involved | Takes a round or two before it clicks |
| `demanding` | Demanding | Expects the rules known up front |

### `socialMode` — how play is structured (per **mode**)

| ID | Label | Use when |
|----|-------|----------|
| `free-for-all` | Free-for-all | Everyone plays for themselves |
| `1v1` | 1v1 | Exactly two players head-to-head |
| `teams` | Teams | Players win or lose as a side |
| `hidden-roles` | Hidden roles | Players hold secret allegiances |
| `co-op` | Co-op | Everyone wins or loses together |

Declared on each mode in `GET /api/v1/game-modes`, not through
`updateMyGameMetadata` — see [seat manifest](#4-seat-manifest-seattemplate) below.

### `typicalMinutes` — how long a round runs (per **mode**, optional)

A whole number of minutes, 1–1440. The catalog card paints it beside the player
count: `12` shows as "12 min", `90` as "1h 30m". A game whose modes declare
different lengths shows the spread, e.g. "5–12 min".

Give your own honest estimate of a typical round — not the fastest possible one,
and not a hard cap. Leave it out if you genuinely don't know yet: a mode without
it simply shows no session length, which is better than a number players will
find wrong. You can add it whenever you next publish.

Declared on each mode in `GET /api/v1/game-modes`, like `socialMode`.

Query `genreTaxonomy`, `difficultyTaxonomy` and `socialModeTaxonomy` for the
machine-readable lists, or call
`joinquest_integration_get_catalog_tag_taxonomy`, which returns all three.

> **Retired (JQ-162).** The `tags` field and its eight ids — `competitive`,
> `cooperative`, `party`, `1v1`, `quick`, `words`, `strategy`, `casual` — are no
> longer writable. Existing games were migrated automatically; nothing to
> re-enter. `quick` is replaced by the per-mode `typicalMinutes` below — a
> number rather than a boolean, so declare it and the card shows the real
> length.

---

## 3. Quick start (API)

1. **Health** — `GET {apiBaseUrl}/healthz` → `ok`
2. **Status** — `GET {apiBaseUrl}/api/v1/status` → `{ game, version, launchUrlsOnProvision: true }`
3. **Game modes** — `GET {apiBaseUrl}/api/v1/game-modes` → modes with valid `seatTemplate`

Register at [/developers](/developers). Lobby syncs your manifest on connect.

---

## 4. Seat manifest (`seatTemplate`)

Each mode needs `minPlayers`, `maxPlayers`, and a `seatTemplate` Lobby expands into join buckets. Games receive a **final seat map** — you don't run matchmaking.

Flat `seats[]` arrays are **rejected**. Use `count` or nested `Team` / role nodes.

Each mode also carries an optional `socialMode` — its
[social shape](#catalog-axes) — and an optional `typicalMinutes`, its
[session length](#typicalminutes--how-long-a-round-runs-per-mode-optional). Both
live here rather than on the game because a game's modes disagree: the same
game's Arena is `free-for-all` and runs about 12 minutes while its Duel is `1v1`
and runs about 5. A value outside 1–1440, or an unknown `socialMode` id, is
rejected at sync; omitting either leaves the mode's current value alone.

```json
{
  "modes": [
    { "key": "arena", "displayName": "Arena", "socialMode": "free-for-all", "typicalMinutes": 12, "seatTemplate": { "count": 8 } },
    { "key": "duel", "displayName": "Duel", "socialMode": "1v1", "typicalMinutes": 5, "seatTemplate": { "count": 2 } }
  ]
}
```

Full reference: [seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md).

---

## 5. Provision contract

`POST {apiBaseUrl}/api/v1/matches`

- **Auth:** `Authorization: Bearer {serviceToken}` from dashboard (or provision body `lobby.serviceToken`)
- **Idempotent** on `assignment.externalMatchId`
- **Success:** `200/201` with `launchUrls` map (lobbyUserId → URL base, no JWT) or `launchUrlTemplate`
- **Banned roster:** `403` with `{ "error": "...", "bannedLobbyUserIds": ["..."] }`

Each seat also carries the player's [skill](#14-player-skill-optional) in the
mode being provisioned, and their [pre-queue picks](#13-pre-queue-options-optional)
if the mode has them.

### Troubleshooting

<a id="provision-403-banned"></a>

**`provision.banlist` fail** — Return 403 with a `bannedLobbyUserIds` string array. To verify, temporarily ban test user `a0000000-0000-4000-8000-000000000099` during checklist runs.

<a id="provision-auth"></a>

**`provision.auth` fail** — Accept the dashboard `serviceToken` on the `Authorization` header.

<a id="provision-launch-urls"></a>

**`provision.launch_urls` fail** — Every seated player in the assignment needs a `launchUrls` entry (or a usable `launchUrlTemplate`).

---

## 6. JWT + claim

Lobby mints seat JWTs (`iss`, `aud` = your API base URL, `matchId`, `seatKey`, `sub` = lobby user id).

- Publish JWKS at `{lobbyIssuer}/.well-known/jwks.json`
- **Claim:** `POST {apiBaseUrl}/api/v1/matches/{externalMatchId}/claim` with `Authorization: Bearer {jwt}`
- Reject wrong `aud`, wrong `iss`, expired tokens, wrong reserved `seatKey`, unknown or mismatched match (`404`), malformed token (`401`/`403`)
- **Claim must be repeatable:** compare the token's `sub` against the player already assigned to that seat. Same `sub` → **`200`, success**; any other `sub` → **`409`**. This is what lets a player who lost their tab get back into their own seat while still keeping anyone else out of it. JoinQuest's checklist exercises both halves.
- **Re-claim must not disturb the match:** the same player returning is "resend me the current state," not "join." No reset, no second join event, no re-deal, no turn advanced. Reply with a full authoritative snapshot rather than the deltas they missed.
- **Key rotation:** Lobby's JWKS can contain more than one active key at once during a signing-key rotation. Verify tokens by matching the JWT's `kid` header against the matching key in the JWKS response — don't cache a single key. If you don't recognize a `kid`, refetch JWKS (rate-limited) before rejecting.

---

### Reconnecting a player

Players close tabs, lose wifi, and take phone calls mid-match. For a casual audience
that is routine, and a player who cannot get back in ruins the match for everyone
still in it. Build **two independent paths back**, so one failing does not strand
anybody:

1. **Your own origin (primary).** On a successful claim, record browser → seat on
   your domain — which `seatKey` of which `externalMatchId`, alongside the verified
   `sub`. A later request with no `?token=` resumes from that binding. This covers a
   refresh, the back button, and a tab crash, with no JoinQuest round trip. Check the
   binding against the match; it names a seat, it does not authorize one.
2. **Through JoinQuest (fallback).** A player returning via JoinQuest gets a
   **Rejoin** action that mints a fresh seat token for the seat they already hold and
   sends them to your launch URL, where the normal claim runs and hits the re-claim
   rule above. This covers a player whose binding with you is gone.

**Return players by navigation, not by POST.** JoinQuest's session cookie is
`SameSite=Lax`: it rides along on a top-level navigation to
`{returnUrl}?match={externalMatchId}`, and is *not* sent on a cross-site POST or a
background `fetch`. A game that hands off by fetching or POSTing gets an anonymous
request and a player who looks logged out.

**While a player is away:** hold their seat, tell the remaining players someone is
disconnected and the game is waiting, and decide up front what happens if they never
return — a forfeit timeout or an abandon vote. An indefinite silent wait is worse for
the people still there than a decided outcome. Do not forfeit on the first dropped
socket, and do not fill the seat.

Full rationale and the wire-level rule: [lobby-protocol-handoff.md](./lobby-protocol-handoff.md#reconnecting-a-player).

---

## 7. Launch URLs (game-minted)

Required. Lobby attaches `token=<jwt>` to each URL base you return. See [game-minted-launch-urls.md](./game-minted-launch-urls.md).

**Do not** embed a JWT in `launchUrls` or `launchUrlTemplate` — JoinQuest adds `token=` when linking players.

---

## 8. Recommended local tests (game repo)

JoinQuest runs remote checks via the developer dashboard or MCP `joinquest_integration_run_game_checks`. **Also add fast unit/integration tests in your game repo** so agents and CI catch regressions before deploy.

See **[reference-games.md](./reference-games.md)** for live demos and GitHub repos. Primary template for tests:

| Repo | Play live |
|------|-----------|
| [ScruffyProdigy/rpslr](https://github.com/ScruffyProdigy/rpslr) | [rpsls-duel.win](https://rpsls-duel.win) |
| [ScruffyProdigy/wordhunt](https://github.com/ScruffyProdigy/wordhunt) | [word-hunt-arena.win](https://word-hunt-arena.win) |

Suggested coverage (rpslr file names shown):

| Area | What to test | Reference (RPSLR) |
|------|----------------|-------------------|
| **Manifest** | `launchUrlsOnProvision: true`; valid `seatTemplate`; expanded `seatKey` list | `app.test.ts`, `gameModes.test.ts` |
| **Provision** | Happy path + `launchUrls` per seat; idempotent re-push; reject missing/invalid `Authorization` | `app.test.ts`, `provision.test.ts` |
| **Launch URLs** | Per-player URLs; no `token=` in bases | `launchUrls.test.ts` |
| **JWT claim** | Valid token; wrong `aud`/`iss`; expired; malformed; URL `:ref` ≠ token `matchId`; wrong reserved seat | `app.test.ts`, `jwksRotation.test.ts` |
| **Re-claim** | Same `sub` re-claiming a held seat gets `200` and unchanged match state; a different `sub` still gets `409`; recovery from your own origin with no `?token=` | `app.test.ts` |
| **JWKS rotation** | Verify tokens signed with either key while both are in JWKS; reject retired key | `jwksRotation.test.ts` |
| **Banlist** | `403` + `bannedLobbyUserIds` when roster includes banned user | `app.test.ts` |
| **Lifecycle** | `reportMatchResult` GraphQL call; return URL builder | `lobbyClient.test.ts`, `lobbyReturn.test.ts` |
| **Realtime** | WS subscribe receives state after REST mutation | `ws.test.ts` |

Agents: after implementing endpoints, add or extend tests mirroring these rows, then run `npm test` in the game API before asking the developer to run JoinQuest checks.

---

## 9. Testing with friends

When integration checks pass:

1. Open your developer dashboard → **Create test table**
2. Invite friends via your room link
3. They can play while the game stays off the public catalog

---

## 10. Public release

When integration checks are green and catalog metadata is complete (`shortDescription`, `longDescription`, at least one tag):

1. Dashboard → **Request public release**
2. JoinQuest reviews (spam/IP/offensive names)
3. Approved games appear in the main catalog

---

## 11. Checklist ID index

| Check ID | Section |
|----------|---------|
| `manifest.reach_api` | §3 Quick start |
| `manifest.status` | §3 Quick start |
| `manifest.launch_urls_on_provision` | §3 Quick start |
| `manifest.game_modes` | §4 Seat manifest |
| `manifest.sync_freshness` | §3 Quick start |
| `provision.happy_path` | §5 Provision |
| `provision.idempotent_repush` | §5 Provision |
| `provision.auth` | [#provision-auth](#provision-auth) |
| `provision.missing_auth` | §5 Provision |
| `provision.banlist` | [#provision-403-banned](#provision-403-banned) |
| `provision.launch_urls` | [#provision-launch-urls](#provision-launch-urls) |
| `provision.launch_url_no_jwt` | §7 Launch URLs |
| `jwt.jwks` | §6 JWT |
| `jwt.claim_happy_path` | §6 JWT |
| `jwt.wrong_audience` | §6 JWT |
| `jwt.unknown_match` | §6 JWT |
| `jwt.wrong_issuer` | §6 JWT |
| `jwt.expired` | §6 JWT |
| `jwt.invalid_token` | §6 JWT |
| `jwt.wrong_seat` | §6 JWT |
| `jwt.reclaim_same_player` | [§6 Reconnecting a player](#reconnecting-a-player) |
| `jwt.reclaim_seat_theft` | [§6 Reconnecting a player](#reconnecting-a-player) |
| `jwt.rotation_overlap` | §6 JWT |

---

## 12. Mode-level eligibility (optional)

Like [`queue-options`](./composition-and-join-options.md#what-belongs-on-the-game-site) (in-queue role choices), **mode-level eligibility** lets your game gate an entire `GameMode` behind player progress the manifest can't express — a tutorial-complete flag, a win-count threshold, a "has a legal deck" check, or a compound requirement. This is a separate, optional mechanism: `queue-options` narrows role choices *within* a mode you can already join; mode-eligibility decides whether the mode is joinable *at all*.

**Opt-in, fail-open if absent.** JoinQuest only calls this endpoint if you implement it. If it 404s, times out, or returns malformed JSON, every mode falls back to `accessible: true` — existing games are unaffected without any changes.

```
GET {apiBaseUrl}/api/v1/players/{lobbyUserId}/mode-eligibility
```

Returns eligibility for **every mode of the game in one payload** (not one call per mode), keyed by `modeKey`:

```json
{
  "modes": {
    "<modeKey>": {
      "accessible": false,
      "reason": "Complete 50 Ranked matches to unlock.",
      "requirement": { "kind": "leaf", "label": "Ranked matches", "current": 12, "target": 50 },
      "unlockModeKey": null
    }
  }
}
```

| Field | Type | Notes |
|-------|------|-------|
| `accessible` | bool | Required. Whether this player can join the mode right now. |
| `reason` | string | Player-facing text shown when locked. Omit or empty when `accessible: true`. |
| `requirement` | object or `null` | Optional progress readout — see below. `null` for a pure boolean gate with nothing countable to show. |
| `unlockModeKey` | string or `null` | Optional. Another mode's `modeKey` to route the player to (e.g. a deck builder). `null` when there's no single obvious next step. |

A mode key you omit from `modes` is treated as `accessible: true` (same as the endpoint being absent entirely).

### `requirement` shapes

A bare **leaf** for a single counter, or a boolean has/has-not check (use `target: 1`):

```json
{ "kind": "leaf", "label": "Ranked matches", "current": 12, "target": 50 }
```

A **group** combining two or more requirements with `"all"` or `"any"` (lowercase; any other value is treated as malformed and fails open):

```json
{
  "kind": "group",
  "label": "Commander requirements",
  "operator": "all",
  "children": [
    { "kind": "leaf", "label": "Standard wins", "current": 18, "target": 25 },
    { "kind": "leaf", "label": "Unique decks used", "current": 3, "target": 5 }
  ]
}
```

`children` nodes are the same leaf/group shape recursively, so nested groups are structurally supported — but JoinQuest's catalog UI only renders one level deep today, so keep requirement trees to a single level of nesting for now.

Query this alongside your modes via GraphQL:

```graphql
query {
  game(id: "…") {
    modes {
      modeKey
      eligibility(playerId: "…") {
        accessible
        reason
        unlockModeKey
        requirement {
          __typename
          ... on RequirementLeaf { label current target }
          ... on RequirementGroup { label operator children { __typename } }
        }
      }
    }
  }
}
```

---

## 13. Pre-queue options (optional)

Some modes ask the player to bring something before matchmaking — a champion, a
loadout, a deck. JoinQuest can render that picker for you, so the choice is made
in the lobby and handed to you with the match.

**Role vs option.** These are different things and integrators conflate them:

| | What it is | Who it affects | Where it is declared |
|---|---|---|---|
| **Role** (`queuePath`) | Which bucket the player queues in — Clue Giver, Attacker | Matchmaking: it decides who they are matched *with* | Derived from `seatTemplate` |
| **Option** (`preQueue`) | What the player brings — a champion, a kit, a deck | Nothing in matchmaking; it is carried to your game | `preQueue` on the mode |

A mode may have either, both, or neither. When it has both, the player picks the
role first and then the options, because which options exist can depend on the
mode.

### Declare the groups in your manifest

Add `preQueue` to a mode in `GET /api/v1/game-modes`:

```json
{
  "key": "duel-helpers",
  "displayName": "Helpers",
  "seatTemplate": { "count": 2 },
  "preQueue": {
    "groups": [
      { "key": "helpers", "kind": "Loadout", "label": "Choose your two helpers", "min": 2, "max": 2 }
    ]
  }
}
```

| Field | Notes |
|-------|-------|
| `key` | Stable identifier for the group; it comes back to you on provision. |
| `kind` | `Character`, `Loadout` or `Deck`. Presentation only — it lets the picker look right without JoinQuest knowing what your options mean. |
| `label` | The player-facing prompt: "Choose your champion", "Bring a deck". |
| `min` / `max` | How many picks this group takes. Defaults to `1` and `1`. `min: 0` makes the group optional. |

More than one group is allowed — a mode can ask for a weapon *and* an armour set.
The declaration only says *what to ask*; it never lists the choices.

### Serve the roster per player

The choices themselves come from you, per player, at request time:

```
GET {apiBaseUrl}/api/v1/players/{lobbyUserId}/queue-options?modeKey=duel-helpers
```

```json
{
  "groups": [
    {
      "key": "helpers",
      "choices": [
        { "id": "ferrus", "label": "Ferrus", "description": "Robot takes 1 mark", "locked": false },
        {
          "id": "rust", "label": "Rust", "locked": true,
          "unlockModeKey": "casual",
          "requirement": { "kind": "leaf", "label": "Casual wins", "current": 3, "target": 5 }
        }
      ]
    }
  ]
}
```

A mode with one declared group may answer with a bare `{"choices": [...]}` and
JoinQuest will adopt it into that group.

Serving the roster live rather than listing it in the manifest is what lets the
roster be genuinely per player: a card game can offer the decks this account
actually built, including one made a minute ago.

**A mode with no options must still answer.** Return `{"groups": []}` — a 404
reads as a broken endpoint, not as "nothing to pick".

### Locking

`locked: true` keeps a choice on the board with its unlock condition showing,
rather than hiding it, so the player can see what they are working toward.
`requirement` is exactly the tree from
[§12](#12-mode-level-eligibility-optional) — same `leaf`/`group` shapes, same
`all`/`any` operators — so option progress renders the way mode progress does.

Locks are yours to decide and need have nothing to do with money: matches
played, a mode completed, an account level, a tutorial finished. **A locked
choice is rejected server-side if a client sends it anyway**, so the lock is real
and not just a disabled button.

### This endpoint does not fail open

Unlike mode-eligibility, JoinQuest will not guess. If a mode declares groups and
this endpoint errors, times out or returns unreadable JSON, **that mode becomes
unjoinable** until it answers, and the player is told why. There is no safe
default: an empty guess would block a legitimate join, and a permissive one would
hand out options you never offered.

Modes without `preQueue` are untouched, and the endpoint is never called for them.

### What you receive

The picks arrive with the match, per seat, in the provision payload:

```json
{
  "assignment": {
    "seats": [
      {
        "seatKey": "p1",
        "lobbyUserId": "…",
        "options": [{ "groupKey": "helpers", "optionIds": ["ferrus", "tempered"] }]
      }
    ]
  }
}
```

`options` is omitted entirely for modes without a pre-queue step, so an existing
game sees an unchanged payload.

Every id in `optionIds` has been checked against the roster you served for that
player — it exists, it was not locked, and the count is inside the `min`/`max`
you declared.

### Group play

At a table, each player answers the picker as they claim their seat; nobody
chooses for anybody else. The picks travel with each player whether the table
starts on its own or backfills through the lobby.

---

## 14. Player skill (optional)

JoinQuest keeps a skill estimate for every player, **per game and per mode**,
derived from the match results you already report. There is nothing extra to
send: the same `reportMatchResult` call that closes out a match is what moves
the number. You are welcome to track your own ratings as well — this is here so
you do not have to.

The estimate is for **your server to use**, not for your players to see.

### What you get

Two channels, both server-to-server, both authenticated with your
`serviceToken`.

**On the provision push**, every seat carries its player's skill in the mode
being provisioned — so you can size a match, pick AI difficulty or balance sides
before it starts, with no extra round trip:

```json
{
  "assignment": {
    "gameMode": "duel",
    "seats": [
      {
        "seatKey": "p1",
        "lobbyUserId": "…",
        "skill": { "rating": 31.5, "uncertainty": 2.25 }
      }
    ]
  }
}
```

**On the GraphQL player lookup**, ask for any player in any mode you declare:

```graphql
query Player($id: ID!) {
  player(id: $id) {
    displayName
    skill(modeKey: "duel") { rating uncertainty }
  }
}
```

### Reading the numbers

| Field | Meaning |
|-------|---------|
| `rating` | Centre of the estimate. A player we have not rated sits at **25.0**; most rated players land roughly between **0 and 50**. Higher is stronger. |
| `uncertainty` | One standard deviation on the same scale — how unsure we are. Widest at about **8.3**, narrowing as evidence accumulates. |

Treat `rating` as a range, not a point: a player at `28.0 ± 7.0` and one at
`28.0 ± 1.5` are not the same information. If you are picking AI difficulty for
someone with a high `uncertainty`, aim near the middle and let the next few
results sharpen it.

**`uncertainty` is the field to branch on**, not how new the player is to you.
There is no match count in the payload on purpose: you already know how many
times you have seen a player id, and that count answers a different question.
How much a number deserves to be trusted is what `uncertainty` is for, and it
keeps meaning that however the estimate was arrived at.

A rating is **only ever comparable inside one game and one mode**. A 30 in your
Duel says nothing about a 30 in your Arena, and nothing at all about a 30 in
somebody else's game.

### What you will never get

- **Another game's rating.** Your `serviceToken` names your game, and that is
  the only catalog entry it reads. A player's standing elsewhere in JoinQuest is
  not yours to see.
- **A mode you do not declare.** `skill` returns `null` for a `modeKey` your
  manifest does not carry, so a typo reads as a mistake rather than as a
  plausible number.
- **Skill on the player's own path.** It is not in the seat JWT and not in any
  field a player's session can read — including a player asking about
  themselves. Every seat token your players hold is decodable in their browser,
  so nothing sensitive rides there.

### Please do not show it to players

JoinQuest deliberately does not show players their own skill, and asks that
games do the same — no number, no bracket, no "you are rated ___" screen.

This is an expectation, not something the lobby can enforce: once the number is
on your server it is yours, and we have no way to check what you render. We are
asking because a visible rating changes how a casual audience plays — it turns a
game you were enjoying into a score you can lose. Use it to make the match
better and leave it out of the UI.

Using it to shape difficulty, seed teams, or pick an opponent is exactly what it
is for. Grading players with it is not.
