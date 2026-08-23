# JoinQuest developer integration guide

> **Embedded copy:** Do not edit here. Canonical source: `docs/developer-integration-guide.md`. Run `./scripts/sync-developer-docs.sh` after editing.


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
| Social mode | tags + tone | competitive, cooperative, or party |
| Session length | tags + copy | quick rounds vs longer sessions |
| Vibe / audience | catalog voice | casual, brainy, chaotic, tactical, etc. |
| API URL | registration | public HTTPS hosting plan (not localhost) |

### From answers → manifest + tags

| If the game is… | Start with | Example tags |
|-----------------|------------|--------------|
| 1v1 competitive | `{ "count": 2 }` duel mode | `competitive`, `1v1`, `quick` |
| Team vs team | `Team: { count: 2, Seat: { count: N } }` | `competitive`, `party` |
| Co-op in one group | `{ "count": N }` or single team template | `cooperative`, `party` |
| Roles / composition | Role buckets in `seatTemplate` | `competitive` or `cooperative` + role-specific copy |

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
| `tags` | Catalog chips | 1–3 from [taxonomy](#catalog-tag-taxonomy); max 3 shown in UI |

---

## 2. Catalog tag taxonomy

Use **only** these tag IDs (labels shown in UI):

| ID | Label | Use when |
|----|-------|----------|
| `competitive` | Competitive | Winners/losers, rankings, direct opposition |
| `cooperative` | Co-op | Players win or lose together |
| `party` | Party | Social, groups, casual fun |
| `1v1` | 1v1 | Exactly two players head-to-head |
| `quick` | Quick | Sessions under ~10 minutes |
| `words` | Words | Word/language puzzles |
| `strategy` | Strategy | Planning, hidden info, tactics |
| `casual` | Casual | Low pressure, easy to pick up |

Query `catalogTagTaxonomy` for the machine-readable list.

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

Full reference: [seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md).

---

## 5. Provision contract

`POST {apiBaseUrl}/api/v1/matches`

- **Auth:** `Authorization: Bearer {serviceToken}` from dashboard (or provision body `lobby.serviceToken`)
- **Idempotent** on `assignment.externalMatchId`
- **Success:** `200/201` with `launchUrls` map (lobbyUserId → URL base, no JWT) or `launchUrlTemplate`
- **Banned roster:** `403` with `{ "error": "...", "bannedLobbyUserIds": ["..."] }`

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
- **Claim must be repeatable:** JoinQuest's checklist claims the same seat more than once during checks (happy path, then again for the rotation check below) — treat a repeat claim of an already-claimed seat by the same player as success, not a conflict.
- **Key rotation:** Lobby's JWKS can contain more than one active key at once during a signing-key rotation. Verify tokens by matching the JWT's `kid` header against the matching key in the JWKS response — don't cache a single key. If you don't recognize a `kid`, refetch JWKS (rate-limited) before rejecting.

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
