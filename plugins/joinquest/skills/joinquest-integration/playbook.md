# JoinQuest agent integration playbook

> **Embedded copy:** Do not edit here. Canonical source: `docs/developer-agent-playbook.md`. Run `./scripts/sync-developer-docs.sh` after editing.


End-to-end workflow for AI agents helping a developer integrate a multiplayer game with JoinQuest — from first conversation through public catalog release.

**Companion docs (load on demand, do not duplicate inline):**

- Technical reference: `developer-integration-guide.md` (MCP: `joinquest_integration_get_integration_guide`)
- Discovery interview (Phase 1 detail): MCP `joinquest_integration_get_discovery_prompt`
- Product spec: `developer-self-service.md`

**Execution:** JoinQuest integration MCP (`joinquest-integration`) — see `mcp-setup.md` in the agent skill folder or the developer dashboard.

---

## Agent rules (always)

1. **Interview before implementing.** Start open-ended — let the developer describe their game, then ask clarifying questions for what's still unclear. Confirm reasonable inferences instead of guessing or re-asking from scratch.
2. **Human gates.** Never call `joinquest_integration_register_game`, `joinquest_integration_update_game_metadata`, `joinquest_integration_connect_game` (when changing `apiBaseUrl`), `joinquest_integration_rotate_webhook_secret`, or `joinquest_integration_request_public_release` without explicit developer approval.
3. **Public HTTPS only.** JoinQuest cannot reach `localhost` — use staging, Render, Fly, ngrok, etc.
4. **One phase at a time.** Finish the current phase (or get explicit skip) before moving on.
5. **MCP first when connected.** Prefer MCP tools over guessing dashboard state. If MCP is not configured, tell the developer how to set it up (Phase 2).
6. **Fail closed, fix forward.** When checks fail, read the error message, pull the integration guide section, fix the game API, re-run checks.

---

## Phase 1 — Discover the game

**Goal:** Understand what the developer is building so you can draft catalog copy and seatTemplate guidance.

**Start open-ended:**

Invite the developer to describe the game in their own words — no checklist yet. For example:

> Tell me about the game you're thinking of — as much or as little as you have. What's the idea, how do people play together, who it's for, anything you're excited about or unsure about.

**Then clarify only what's missing:**

Read their description and identify gaps. You need enough to draft registration copy and a seatTemplate plan. Ask follow-ups conversationally — one or two at a time, not a wall of questions. If they already answered something in their opening description, **do not re-ask it**.

**Confirm what you think you know:** If you can infer an answer but aren't fully sure, check it with the developer instead of guessing or asking from scratch. For example: "It sounds like this is mostly a 2-player game — would you say that's fair?" Same for structure, vibe, session length, and the rest.

| Topic | Why it matters | Only ask if unclear |
|-------|----------------|---------------------|
| Player count | seatTemplate / game-modes | min/max, fixed or variable |
| Structure | seatTemplate | duel, free-for-all, teams, or roles |
| Genre | `genre` | action, strategy, deduction, words & trivia, drawing & creative, or puzzle |
| Social mode | `socialMode` on each mode | free-for-all, 1v1, teams, hidden roles, or co-op — ask per mode, they often differ |
| Session length | `typicalMinutes` on each mode | roughly how long one round runs — ask per mode, a Duel is usually shorter than an Arena |
| Difficulty | `difficulty` | how much a new player must know before their first round is fun |
| Vibe / audience | catalog voice | brainy, chaotic, tactical, etc. — copy, not a field |
| API URL | registration | public HTTPS hosting plan (not localhost) |

**Draft (show to developer, do not save yet):**

| Output | Constraints |
|--------|-------------|
| `shortDescription` | ~120 chars, warm JoinQuest tone — what players *do*, not tech stack |
| `longDescription` | 2–4 short paragraphs for the detail page |
| `howToPlay` | 3–6 bullet steps for a first-time player |
| `genre` | exactly one id from `joinquest_integration_get_catalog_tag_taxonomy` → `genre` |
| `difficulty` | optional; one id from the same tool's `difficulty` list |
| `socialMode` | one id per **mode**, declared in the game's `/api/v1/game-modes` — not in metadata |
| `typicalMinutes` | optional; whole minutes (1–1440) per **mode**, declared in `/api/v1/game-modes` — not in metadata |
| `seatTemplate` plan | Point to integration guide §4 + seat-templates cookbook (duel, teams, roles) |

**Voice:** Plain, player-first. “Find your group. Play together.” — not “enter matchmaking” or JWT jargon.

**Done when:** Developer confirms the drafts are directionally correct (edits OK). You have enough to register the game and suggest a seatTemplate.

---

## Phase 2 — Connect JoinQuest MCP

**Goal:** Agent can call JoinQuest on the developer's behalf.

**Developer actions (guide them):**

1. Sign in at [joinquest.cc/developers](https://joinquest.cc/developers).
2. Developer dashboard → **Connect an AI assistant** → **Show setup** → **Generate API key** (copy once).
3. Install MCP CLI is **not required** — configs use `npx -y @joinquest/mcp-integration` (Node.js 20+).

4. Add MCP config — **Cursor** (note the cursor-specific npx bin):

```json
{
  "mcpServers": {
    "joinquest-integration": {
      "type": "stdio",
      "command": "npx",
      "args": ["--yes", "--package", "@joinquest/mcp-integration", "joinquest-integration-mcp-cursor"],
      "env": {
        "JOINQUEST_API_KEY": "lq_dev_..."
      }
    }
  }
}
```

**Claude Code:** `claude mcp add --scope project --transport stdio --env JOINQUEST_API_KEY=lq_dev_... joinquest-integration -- npx -y @joinquest/mcp-integration`

Config paths: Cursor → `.cursor/mcp.json`; Claude Code → `.mcp.json`; Copilot → `.vscode/mcp.json`; Roo → `.roo/mcp.json`; Windsurf → `~/.codeium/windsurf/mcp_config.json`; Cline → Cline MCP settings. **ChatGPT:** not supported (needs hosted HTTPS MCP) — use [Register in the browser](https://joinquest.cc/developers?path=manual) instead.

5. **Fully quit** the agent client and reopen (Cursor: **Cmd+Q**, not just Reload Window). Start a **new Agent chat**.

**Optional — one-line install from game repo** (skill + MCP + platform rules):

```bash
JOINQUEST_API_KEY=lq_dev_... curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/joinquest/main/scripts/install-joinquest-dev.sh | sh -s -- --cursor
```

Flags: `--claude`, `--claude-desktop`, `--copilot`, `--roo`, `--windsurf`, `--cline`, or `--all` (Cursor + Claude Code). Skill-only: `--skill-only`.

**Verify MCP:**

In Agent mode, ask the agent to call `joinquest_integration_list_my_games`. If auth fails, re-check API key and URL. Some Cursor versions have no **Tools & MCP** settings tab — a successful tool call is the real test.

**Done when:** MCP tools respond successfully.

---

## Phase 3 — Implement the game API

**Goal:** Game server exposes the endpoints JoinQuest calls. Implement in the **game repo**, not the lobby repo.

**Required endpoints:**

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Returns `ok` — JoinQuest probes reachability |
| `GET /api/v1/status` | Game name, version, `launchUrlsOnProvision: true` |
| `GET /api/v1/game-modes` | Modes with `minPlayers`, `maxPlayers`, valid `seatTemplate` (no flat `seats[]`) |
| `POST /api/v1/matches` | Provision — auth via `Authorization: Bearer {serviceToken}` |
| `POST /api/v1/matches/{externalMatchId}/claim` | Validate Lobby seat JWT |

**Provision success:** `200/201` with `launchUrls` map (lobbyUserId → URL base) or `launchUrlTemplate`.

**Provision banlist:** `403` with `{ "error": "...", "bannedLobbyUserIds": ["..."] }`.

**JWT:** Verify `iss`, `aud` (your API base URL), `matchId`, `seatKey`, `sub`. Publish JWKS at `{lobbyIssuer}/.well-known/jwks.json`.

**Optional per-player endpoints:**

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/players/{lobbyUserId}/mode-eligibility` | Gate whole modes on progress. Fails open — a silent game means every mode is playable. Guide §12 |
| `GET /api/v1/players/{lobbyUserId}/queue-options?modeKey=…` | Serve the champion/loadout/deck roster for a mode that declares `preQueue`. Guide §13 |

**Role vs option — do not conflate them.** A **role** (`queuePath`, derived from `seatTemplate`)
decides who the player is matched *with*. An **option** (`preQueue` on the mode) is what the player
*brings* and changes nothing about matchmaking. A mode can have either, both, or neither; when it
has both, the role is chosen first.

Two things about `queue-options` catch people out:

- It **does not fail open.** A mode declaring `preQueue` whose roster cannot be loaded becomes
  unjoinable, deliberately — see guide §13.
- A mode with nothing to pick must return `{"groups": []}`, not a 404.

Use Phase 1 seatTemplate plan. Full details: integration guide §3–§8. Pick a **reference game** ([reference-games.md](./reference-games.md)) for API layout and client boot patterns — duel → [rpslr](https://github.com/ScruffyProdigy/rpslr); party/multi-seat → [wordhunt](https://github.com/ScruffyProdigy/wordhunt).

**Local tests:** Add game-repo tests mirroring integration guide §8 (manifest, provision, JWT, JWKS rotation). Use [rpslr](https://github.com/ScruffyProdigy/rpslr) as the primary test template. Run `npm test` before remote JoinQuest checks.

**Done when:** Endpoints exist on a **public HTTPS** URL, local tests pass, and basic manual curl tests pass.

---

## Phase 4 — Register the game on JoinQuest

**Goal:** Game row exists; manifest sync attempted.

**Registration fields** — confirm values with the developer (Phase 1 drafts), then register via MCP or [/developers](https://joinquest.cc/developers):

| Field | Required | Notes |
|-------|----------|-------|
| Game name | Yes | Display name |
| Slug | Yes | Lowercase URL slug (auto-suggested from name; **immutable** after register) |
| Short description | Yes | Initial card blurb (~1–2 sentences) |
| API base URL | Yes | Public HTTPS origin, e.g. `https://mygame.example.com` |
| Contact email | Yes | Review notifications |
| Website URL | No | |
| Community URL | No | Discord, etc. |

**MCP:** `joinquest_integration_register_game` with the fields above. JoinQuest probes `healthz` + `game-modes` on register.

**After register:** Success → `private_testing`. Failure → stays `draft` with `connectError`.

**If draft / connect failed:** fix the public API, then MCP `joinquest_integration_connect_game` (optional new `apiBaseUrl`) or dashboard **Connect API**. Do not re-register.

**If API host changes later** (staging → prod): `joinquest_integration_connect_game` with the new URL — only while `draft` or `private_testing` (blocked for pending review / public).

**MCP (after register):**

- `joinquest_integration_list_my_games` — confirm `gameId`
- `joinquest_integration_get_game_checks` — current state
- `joinquest_integration_get_game_credentials` — `serviceToken`, `webhookSecret` (sensitive)
- `joinquest_integration_get_example_provision_payload` — sample POST body for local debugging

**Done when:** Game visibility is `PRIVATE_TESTING` (or developer understands draft errors and is fixing via connect).

---

## Phase 5 — Run integration checks and fix failures

**Goal:** All required checks pass.

**When the game API’s seatTemplate / modes change:** call `joinquest_integration_sync_game_manifest` **before** re-running checks. Live checks can pass against a new API while JoinQuest still has a stale cached manifest — sync refreshes modes/seats. Dashboard: **Resync game modes**.

**MCP:** `joinquest_integration_run_game_checks` with `gameId`.

**Required checks (19 — must be PASS for public release):**

- **Manifest:** `manifest.reach_api`, `manifest.status`, `manifest.launch_urls_on_provision`, `manifest.game_modes`, `manifest.sync_freshness`
- **Provision:** `provision.happy_path`, `provision.idempotent_repush`, `provision.auth`, `provision.missing_auth`, `provision.launch_urls`, `provision.launch_url_no_jwt`
- **JWT:** `jwt.jwks`, `jwt.claim_happy_path`, `jwt.wrong_audience`, `jwt.unknown_match`, `jwt.wrong_issuer`, `jwt.expired`, `jwt.invalid_token`, `jwt.wrong_seat`

**Optional (recommended, not required for release):** `provision.banlist` — skipped unless the game bans test user `a0000000-0000-4000-8000-000000000099`. `jwt.rotation_overlap` — verifies your game accepts a token signed with a newly added JWKS key while an older key is still active; fails if you cache a single key instead of matching by `kid`.

**On failure:**

1. Read `message` and `detail` from the check result.
2. Load integration guide §11 checklist index → relevant section; add local tests per §8 in the game repo.
3. Fix the **game API**, deploy, **`joinquest_integration_sync_game_manifest`**, then re-run checks.

Common fixes:

| Check | Fix |
|-------|-----|
| `manifest.reach_api` | API must be public HTTPS; `/healthz` returns 200 |
| `manifest.launch_urls_on_provision` | Set `launchUrlsOnProvision: true` on `GET /api/v1/status` |
| `manifest.game_modes` | Valid `seatTemplate` JSON per mode; then **sync** |
| `manifest.sync_freshness` | `joinquest_integration_sync_game_manifest` or `connectMyGame` |
| `provision.auth` | Accept dashboard `serviceToken` on `Authorization: Bearer …` |
| `provision.missing_auth` | Reject provision with no (or wrong) `Authorization` header |
| `provision.idempotent_repush` | Re-posting the same `externalMatchId` must succeed |
| `provision.launch_urls` | Return `launchUrls` for every seated player |
| `provision.launch_url_no_jwt` | Do not embed JWTs in launch URL bases |
| `provision.banlist` | Return 403 + `bannedLobbyUserIds` array (optional check) |
| `jwt.jwks` | Verify tokens via `{iss}/.well-known/jwks.json` |
| `jwt.claim_happy_path` | `POST …/claim` accepts valid seat JWT |
| `jwt.wrong_audience` | Reject tokens whose `aud` ≠ your API base URL |
| `jwt.unknown_match` | Claim on unknown or URL-mismatched match → 404 |
| `jwt.wrong_issuer` | Reject tokens whose `iss` ≠ provision `lobbyId` |
| `jwt.expired` / `jwt.invalid_token` | Reject expired or malformed tokens → 401 |
| `jwt.wrong_seat` | Reject tokens for another player's reserved seat |
| `jwt.rotation_overlap` | On unrecognized `kid`, refetch JWKS (rate-limited) before rejecting — don't cache a single key (optional check) |

**Done when:** All 19 required checks are green (re-run to confirm).

---

## Phase 6 — Save catalog metadata

**Goal:** Listing copy matches Phase 1 drafts (updated if needed).

**MCP:** `joinquest_integration_get_catalog_tag_taxonomy` for valid tag IDs.

**Show developer the final draft.** On explicit approval:

**MCP:** `joinquest_integration_update_game_metadata` with:

```json
{
  "gameId": "...",
  "name": "...",
  "shortDescription": "...",
  "longDescription": "...",
  "howToPlay": "...",
  "genre": "words-trivia",
  "difficulty": "casual",
  "contactEmail": "dev@example.com",
  "websiteUrl": "https://...",
  "communityUrl": "https://discord.gg/...",
  "accentColor": "#7c3aed"
}
```

Empty `websiteUrl` / `communityUrl` / `accentColor` / `genre` / `difficulty` clears those optional fields. Slug is not editable.

`socialMode` and `typicalMinutes` are deliberately absent here: both are
properties of a mode, not of the game, so they are declared in
`GET /api/v1/game-modes` alongside `seatTemplate`. A game whose Arena is
free-for-all and runs 12 minutes, and whose Duel is 1v1 and runs 5, cannot say
either with one game-level value.

Take `typicalMinutes` from the developer rather than guessing it, and leave it
out when they are not sure — an omitted duration shows no session length on the
catalog card, while a wrong one is a promise to players that the game breaks.

The flat `tags` field was retired in JQ-162 — writing it is no longer possible,
and existing games were migrated onto the axes automatically.

**Credentials:** If `webhookSecret` leaks, `joinquest_integration_rotate_webhook_secret` (human approval). `serviceToken` is derived from game id — not rotatable.

**Done when:** Metadata saved and visible via `joinquest_integration_get_game_checks`.

---

## Phase 7 — Test with friends

**Goal:** Real players can join a table in a private room.

**Developer dashboard:** **Create test table** (or equivalent flow).

Friends join via the developer's room link. Game stays off the public catalog until release.

**Agent role:** Confirm checks are green; encourage a playtest; if the client is still a stub, continue **Phase 9** until the session is actually fun to play.

**Done when:** Developer confirms a successful test session (or explicitly skips).

---

## Phase 8 — Request public release

**Goal:** Submit for JoinQuest catalog review.

**Gates (automated):**

- Required checks PASS
- `shortDescription`, `longDescription`, at least one tag
- Visibility `PRIVATE_TESTING`

**Confirm with developer:** They want to go public.

**MCP:** `joinquest_integration_request_public_release` with `gameId`.

**After submit:** Visibility → `PENDING_REVIEW`. JoinQuest reviews for spam/IP/offensive content.

**Done when:** Request submitted (or game already public).

---

## Phase 9 — Build the playable game (beyond the handshake)

**Goal:** Players get an **immersive, fun session** at the launch URLs — not just a stub that passes checks.

JoinQuest integration (Phases 3–5) proves the **wire contract**. Phase 9 is everything that makes people want to come back: rules, feel, clarity, pacing, and polish in **your game repo** (client + any game-specific server logic).

**When to start:** As soon as the API skeleton exists (Phase 3), sketch the client in parallel. After checks are green (Phase 5), shift focus here before asking for public release.

**Reference games** — read or clone the closest match ([reference-games.md](./reference-games.md)):

| If the game is… | Start with | Why |
|-----------------|------------|-----|
| 1v1 duels, first integration | [rpslr](https://github.com/ScruffyProdigy/rpslr) | Minimal duel API + client + tests |
| Party / multi-seat, richer UI | [wordhunt](https://github.com/ScruffyProdigy/wordhunt) | Path launch URLs, sync, party flow |

Play live first when you can: [rpsls-duel.win](https://rpsls-duel.win), [word-hunt-arena.win](https://word-hunt-arena.win).

**Agent role (stay in the game repo):**

1. **Boot from JoinQuest** — client reads `token` (and match/seat from URL); claim or validate seat JWT; show who you’re playing with.
2. **Core loop** — implement the rules and interactions from Phase 1; keep sessions short enough for “one more round.”
3. **Multiplayer feel** — realtime or turn sync; clear whose turn; graceful disconnect; win/lose/draw states.
4. **Return path** — “Back to JoinQuest” using the same patterns as reference games (`lobbyReturn`, match result callbacks per integration guide §8).
5. **Playtest loop** — developer runs a test table (Phase 7); fix confusion and bugs; **update catalog copy** if the built game differs from Phase 1 drafts before Phase 6/8.
6. **Polish when asked** — sound, animation, copy tweaks — after the loop works; don’t block integration on polish.

**Do not** stop helping after checks pass. **Do** separate JoinQuest dashboard ops (MCP) from game implementation (game repo). If the developer pivots to gameplay, follow their lead while keeping launch URLs and handoff working.

**Done when:** Developer confirms friends enjoyed a real session (Phase 7) and the experience matches the catalog promise (or metadata was updated to match).

---

## MCP tool quick reference

| Phase | Tools |
|-------|-------|
| Setup | `joinquest_integration_get_agent_playbook` (this doc), `joinquest_integration_get_integration_guide` |
| Discover | `joinquest_integration_get_discovery_prompt`, `joinquest_integration_get_catalog_tag_taxonomy` |
| Register / status | `joinquest_integration_register_game`, `joinquest_integration_connect_game`, `joinquest_integration_sync_game_manifest`, `joinquest_integration_list_my_games`, `joinquest_integration_get_game_checks`, `joinquest_integration_get_game_credentials`, `joinquest_integration_rotate_webhook_secret`, `joinquest_integration_get_example_provision_payload` |
| Checks | `joinquest_integration_run_game_checks` (after sync when modes changed) |
| Metadata | `joinquest_integration_update_game_metadata` |
| Release | `joinquest_integration_request_public_release` |
| Build game | Reference repos — [reference-games.md](./reference-games.md); no MCP substitute for gameplay code |

---

## Phase checklist (copy for status updates)

```text
[ ] Phase 1 — Discover game (developer described idea; drafts approved)
[ ] Phase 2 — MCP connected
[ ] Phase 3 — Game API + local tests (public HTTPS; integration guide §8)
[ ] Phase 4 — Registered on JoinQuest (private_testing)
[ ] Phase 5 — Integration checks passing
[ ] Phase 6 — Catalog metadata saved
[ ] Phase 7 — Test table played (fun, not just green checks)
[ ] Phase 8 — Public release requested
[ ] Phase 9 — Playable game client (immersive loop; metadata matches reality)
```
