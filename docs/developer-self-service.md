# Developer self-service — game registration & integration

**Status:** Phase A shipped · Phase B largely shipped (integration checks, MCP, agent discovery)  
**Next:** public release review polish, scheduled re-checks  
**Related:** [developer-integration-guide.md](./developer-integration-guide.md) · [game-minted launch URLs](./game-minted-launch-urls.md) ✅ · [player experience roadmap](./player-experience-roadmap.md)

Today, adding a game means backfilling the database or calling admin-only `registerGame`. That works for us, but it blocks the goal of **any web developer** plugging in their game. This spec is the v1 path from homepage curiosity → registered game → working integration → friends testing → public release.

---

## Product principles

1. **Register freely; list publicly when ready.** Integration is open. Showing up in the main player catalog is reviewed — so players see real titles, not half-finished experiments.
2. **Fail closed with friendly errors.** If we can't reach your API or provision fails, we say *why* and what to fix — not “something went wrong.”
3. **Private until you're proud.** Your game isn't mixed into the public arcade until you ask and we approve.
4. **Agents welcome.** The same checklist humans see in the dashboard should be runnable by Claude/Cursor via MCP — happy paths and error paths — without the developer clicking through docs.

---

## v1 scope decisions

| Topic | v1 | Later |
|-------|-----|-------|
| Ownership | One JoinQuest user owns each game | Studios / teams (v2) |
| Auth | Same magic-link account as players | Separate dev portal optional |
| API URL | Public HTTPS only — **no `localhost`** | Optional tunnel/CLI helper (v2+) |
| Play URL | From game provision response ([spec](./game-minted-launch-urls.md)) | — |
| Player counts | From manifest | — |
| Icons / tags | Agent-assisted draft + owner edit; tags from fixed taxonomy | Custom tag vocabulary (v2) |
| PDF guide | Hold or generate from same source as agent doc | Phase C |

**Why no localhost:** Lobby servers must reach the game API to sync manifests and provision matches. A URL only you can hit from your laptop can't work in production. Devs should use a staging deploy, ngrok, or similar for early integration — we'll say that plainly in copy and errors.

---

## Visibility model (recommendation)

**Do not show private games in the public player catalog.** That list is for games anyone can queue into. Mixing in “my unfinished thing” confuses players and makes the homepage feel broken.

Instead, use **two surfaces**:

```text
Homepage (signed in)
├── Public catalog          →  status = public, checklist green
└── “Your games” strip      →  owner’s games (draft / testing / pending review)
        └── link to Developer dashboard

Developer dashboard (/developers/games/:id)
├── Checklist + errors
├── Catalog listing (short/long copy, tags, how to play) — agent drafts, dev approves
├── Integration credentials (serviceToken, webhook secret)
└── Actions: test table, request public release

Room (invite link)
└── Tables for any game the room members are allowed to play
    (owner’s private games + public games)
```

| Surface | Who sees private games |
|---------|----------------------|
| Public `games` query / main catalog | Nobody |
| Homepage “Your games” (owner signed in) | Owner |
| Developer dashboard | Owner |
| Room tables | Everyone in that room |

**Table rule:** If the owner creates a table for their game, **everyone in the room** can see and join that table — same as today. The game stays off the public catalog; the room is the share link for friends.

**`createPrivateTable`** already creates a room when the user doesn't have one — onboarding can say “Create a test table” as one step, not three.

---

## User-facing copy (casual / friendly)

### Homepage CTA

Link text:

> **Making a game?** We'd love to host it — [get started →](/developers)

### Landing page (`/developers`)

**Headline:** Have an idea for a multiplayer game?

**Body:**

> JoinQuest handles the boring stuff — player accounts, rooms, tables, matchmaking, finding players — so you can focus on building the game.
>
> You'll need some basic web dev experience and a **public URL** where your game can run. That's it.
>
> Register below to get started. You're not committing to anything, and your game won't show up for other players until **you** say it's ready (and we've had a quick look).

### Registration form

**Required**

- Game name  
- Short description (1–2 sentences for your catalog card later)  
- API base URL (HTTPS origin, e.g. `https://mygame.example.com`)  
- Contact email (spec updates & review — can differ from sign-in email)  
- Slug (auto-suggested from name, editable)

**Optional**

- Website  
- Discord (or other community link)

**Not on form**

- `playUrl` — game returns launch URLs on provision  
- Min/max players — from manifest  
- Icons / tags — follow-up or pre-work item

**Submit behavior (two-phase)**

1. **Save draft** — name, slug, contact, URLs stored; game `visibility = draft`.  
2. **Connect API** — Lobby calls `healthz` + `game-modes`; on success → `visibility = private_testing` and dashboard unlocks. On failure → stay draft, show actionable error (“We couldn't reach `https://…` — is it up? We can't use localhost.”).

### Post-registration (`/developers/games/:id/welcome`)

> **Nice — your game is registered.**
>
> Next up: teach your game how to talk to JoinQuest.
>
> **Using Claude, Cursor, or another AI assistant?** Turn on the JoinQuest integration MCP and point your agent at our integration guide — it can run the same checks as the dashboard and fix issues for you.
>
> **Prefer doing it yourself?** Same guide works for humans — step by step, no agent required.
>
> When things look good, spin up a **test table** and invite friends into your room. They can play before your game goes public.
>
> Ready for the world? Hit **Request public release** on your dashboard when the checklist is green.

### Public listing expectation (footer on dashboard + vision)

> Registering is free and instant. **Showing up in the main JoinQuest catalog** means a quick review — mostly so names aren't spam, offensive, or obvious IP issues. Your game still runs on your site either way.

---

## Game lifecycle states

```text
draft
  →  (API connect succeeds)  →  private_testing
  →  (developer requests)    →  pending_review
  →  (admin approves)        →  public
  →  (admin rejects)         →  private_testing  (with reason)
```

| State | Public catalog | Owner dashboard | Room tables |
|-------|----------------|-----------------|-------------|
| `draft` | Hidden | Yes | No (manifest not synced) |
| `private_testing` | Hidden | Yes | Yes (owner + room members) |
| `pending_review` | Hidden | Yes (read-only release btn) | Yes |
| `public` | Listed | Yes | Yes |

**v1 ownership:** `games.owner_user_id` → registering user. Admin (`LOBBY_ADMIN_EMAILS`) retains override + `registerGame` for internal titles.

---

## Registration & catalog (technical)

Extend today’s admin `registerGame` into owner-scoped **`registerMyGame`** (or same mutation with auth branch):

- Input aligns with form above (no `playUrl` once game-minted URLs ship).  
- Output: `game`, `webhookSecret`, `serviceToken` (same as today).  
- Manifest sync: existing `RegisterGame` / `ApplyGameManifest` pipeline.  
- `ListCatalogGames`: only `visibility = public` and active mode queues.  
- New: `myGames` query for owner dashboard + homepage strip.

**Contact email:** `games.contact_email` — notifications for spec changes and review outcomes.

**Optional links:** `website_url`, `community_url` (Discord etc.).

---

## Developer dashboard

Route: `/developers/games/:id` (signed-in owner only).

### Checklist sections

Each row: **status** (pass / fail / not run), **last checked**, **plain-language explanation**, **technical detail** (expandable), **Re-run** button.

#### 1. Manifest

| Check | Pass | Fail message (example) |
|-------|------|-------------------------|
| Reach API | `GET {apiBaseUrl}/healthz` → `ok` | “We couldn't reach your server. Check the URL and that it's HTTPS and publicly reachable (not localhost).” |
| Status | `GET /api/v1/status` | “Your game didn't return a status payload. See integration guide § Status.” |
| Launch URLs on provision | `launchUrlsOnProvision: true` in status | “Set `launchUrlsOnProvision: true` on `/api/v1/status` when your game mints per-player launch URLs.” |
| Game modes | `GET /api/v1/game-modes` valid `seatTemplate` | “Your seat manifest has an error: …” |
| Sync freshness | Manifest hash / ETag | “Your cached manifest is stale — …” |

#### 2. Provisioning

| Check | Pass | Fail message (example) |
|-------|------|-------------------------|
| Happy path | `POST /api/v1/matches` 200/201 with `launchUrls` | “Provision failed: {status} — …” |
| Idempotent re-push | Same `externalMatchId` succeeds again | “Re-provisioning the same match should succeed (idempotent).” |
| Auth | Bearer `serviceToken` accepted | “Your server rejected our service token. Compare with dashboard credential.” |
| Missing auth | No `Authorization` header → 401/403 | “Provision without auth should be rejected when a service token is configured.” |
| Banlist | `403` + `bannedLobbyUserIds` shape | “When banning a test user, return 403 with `bannedLobbyUserIds` array — we got …” |
| Launch URLs | Response includes per-player `launchUrls` (required when opted in; catalog fallback if omitted) | “Provision succeeded but didn't return launch URLs for all seats.” |
| No JWT in launch URLs | `launchUrls` / template must not contain a JWT | “Do not embed JWTs in launch URLs — JoinQuest adds `token=` when linking players.” |

#### 3. JWT verification

| Check | Pass | Fail message (example) |
|-------|------|-------------------------|
| JWKS | Lobby JWKS reachable from game’s perspective (or we document game-side fetch) | “We couldn't verify JWKS setup — …” |
| Claim happy path | Test token → `POST …/claim` 200 | “Claim failed: …” |
| Wrong audience | Token with bad `aud` rejected | “Your game accepted a token with the wrong audience — `aud` must be your API base URL.” |
| Unknown match | Claim on bogus match → 404 | “Unknown match should return 404, got …” |
| Wrong issuer | Token `iss` ≠ provision `lobbyId` → 401 | “Token issuer must match the match's provisioned `lobbyId`.” |
| Expired token | Past `exp` → 401 | “Expired seat tokens must be rejected.” |
| Invalid token | Malformed JWT → 401 | “Malformed tokens must be rejected.” |
| Wrong seat | Token for another reserved seat → 401/403 | “A user cannot claim a seat reserved for someone else.” |
| Rotation overlap | Token signed under a temporary second key is accepted while both keys are active in JWKS (optional, non-gating) | “Your game rejected a token signed with a newly added key while the old key was still active — verify tokens by matching the JWT's `kid` header against every key in the JWKS response, not just the first/cached one.” |

**Implementation note:** Run checks server-side on demand and on a schedule (e.g. every few hours for `private_testing` / `public` games). Store last result JSON on `game_integration_checks` or similar.

### Dashboard actions

- Copy `serviceToken`, webhook secret (masked + reveal)  
- **Edit catalog listing** — short/long description, how to play, tags (taxonomy-backed)  
- **Run all checks**  
- **Create test table** → `createPrivateTable` + open room panel  
- **Request public release** (enabled when required checks pass + catalog metadata complete)

---

## Agent-assisted catalog metadata (Phase B)

Many lobby UI fields benefit from an agent that **interviews the developer**, drafts copy in **JoinQuest voice**, and saves via the same API the dashboard uses.

### Flow

```text
Discovery (agent ↔ developer)
  →  Draft metadata + seatTemplate guidance
  →  Developer approves / edits in dashboard or chat
  →  updateMyGameMetadata mutation
  →  Technical checks (manifest / provision / JWT)
  →  Request public release
```

### Discovery script

Agents call `developerDiscoveryPrompt` (or read integration guide §0).

**Start open-ended** — invite the developer to describe their game in their own words (idea, how people play together, audience, open questions). **Do not lead with a numbered checklist.**

**Then clarify only what's missing** — player count, structure (duel/teams/roles), social mode, session length, vibe, API URL. Ask one or two follow-ups at a time; skip anything they already covered. If you can infer an answer but aren't fully sure, confirm it: e.g. "It sounds like this is mostly a 2-player game — would you say that's fair?"

From the conversation, the agent drafts:

| Output | Constraints |
|--------|-------------|
| `shortDescription` | ~120 chars, catalog card, JoinQuest tone |
| `longDescription` | 2–4 paragraphs for detail page |
| `howToPlay` | 3–6 bullet steps for new players |
| `tags` | 1–3 IDs from `catalogTagTaxonomy` |
| `seatTemplate` notes | Point to cookbook pattern (duel, 3v3, composition) — implemented on game API |

**Agents do not publish without developer confirmation.** MCP and dashboard both use `updateMyGameMetadata`.

### Catalog voice

Warm, plain, player-first — “Find your group. Play together.” not “enter matchmaking.” No JWT/provision jargon on catalog cards. Full rules: [developer-integration-guide.md §1](./developer-integration-guide.md#1-catalog-voice-joinquest-tone).

### Tag taxonomy (v1)

Fixed IDs (not freeform): `competitive`, `cooperative`, `party`, `1v1`, `quick`, `words`, `strategy`, `casual`. UI shows human labels; max 3 chips on cards.

Query: `catalogTagTaxonomy`. Validate on `updateMyGameMetadata`.

### GraphQL (owner-scoped)

```graphql
catalogTagTaxonomy { id label description }
developerDiscoveryPrompt  # markdown interview script
updateMyGameMetadata(input: { gameId, shortDescription, longDescription, howToPlay, tags })
```

---

## Public release

**Developer prerequisites (automated gate):**

- Required checklist items green (manifest + provision; JWT where not skipped)  
- Game name set  
- `shortDescription`, `longDescription`, and at least one valid tag  
- HTTPS API URL  
- (Optional later) custom icon present  

**Human review (admin queue):**

- Name not junk / offensive / obvious infringement  
- Description reasonable  
- Approve → `visibility = public`  
- Reject → back to `private_testing` with reason email to `contact_email`

**Admin UI v1:** Could be GraphQL + script or minimal `/admin/games/pending` — detail TBD.

---

## JoinQuest integration MCP (Phase B)

Goal: agent runs the **same probes** as the dashboard without the developer in the loop.

**Auth:** Developer API key (`lq_dev_…`) from the dashboard, or session cookie. MCP uses `JOINQUEST_API_KEY`.

**Tools (sketch):**

| Tool | Purpose |
|------|---------|
| `joinquest_integration_get_agent_playbook` | Returns end-to-end agent workflow (discovery → release) |
| `joinquest_integration_get_integration_guide` | Returns markdown guide (single source of truth) |
| `joinquest_integration_get_discovery_prompt` | Open-ended discovery prompt + follow-up guide (guide §0) |
| `joinquest_integration_get_catalog_tag_taxonomy` | Valid tag IDs + labels |
| `joinquest_integration_list_my_games` | Owner's games + states |
| `joinquest_integration_register_game` | Register a new game (after developer confirms fields) |
| `joinquest_integration_get_game_checks` | Latest checklist results + errors |
| `joinquest_integration_run_game_checks` | Run manifest / provision / JWT suite (19 required checks) |
| `joinquest_integration_sync_game_manifest` | Re-fetch modes/seats after API / seatTemplate fixes |
| `joinquest_integration_connect_game` | Connect / update apiBaseUrl (draft + private_testing) |
| `joinquest_integration_rotate_webhook_secret` | Rotate webhook secret |
| `joinquest_integration_update_game_metadata` | Save catalog copy + tags (after dev approval) |
| `joinquest_integration_get_game_credentials` | serviceToken, webhook URL (masked) |
| `joinquest_integration_get_example_provision_payload` | Sample assignment for copy-paste |
| `joinquest_integration_request_public_release` | Submit for review when gates pass |

MCP calls backend GraphQL — **one implementation**, two clients (UI + MCP).

**Implementation:** [`mcp/joinquest-integration/`](../../mcp/joinquest-integration/) — stdio server, `JOINQUEST_API_KEY` auth.

**Agent skill:** [`.agents/skills/joinquest-integration/`](../../.agents/skills/joinquest-integration/) — cross-platform `SKILL.md` + bundled playbook (Claude Code, Codex, Cursor, Copilot, etc.).

**Agent playbook (markdown):** [developer-agent-playbook.md](./developer-agent-playbook.md) — also served via GraphQL `developerAgentPlaybook` and MCP.

**Cursor/Claude setup:** [Developer dashboard](https://joinquest.cc/developers) → **Connect an AI assistant** (per-editor tabs). **Plugins:** Cursor / Claude Code via `install-joinquest-cursor-plugin.sh` and `install-joinquest-claude-plugin.sh`. **Other editors:** `install-joinquest-dev.sh --copilot` / `--roo` / `--windsurf` / `--cline` from your game repo. **ChatGPT:** no MCP — [register in the browser](https://joinquest.cc/developers?path=manual). Reference: [`mcp/joinquest-integration/README.md`](../../mcp/joinquest-integration/README.md) and [`.agents/skills/joinquest-integration/mcp-setup.md`](../../.agents/skills/joinquest-integration/mcp-setup.md).

---

## Integration guide (single source of truth)

**File:** [developer-integration-guide.md](./developer-integration-guide.md)

**Consumers:**

- Human-readable web page (`/developers/guide`)  
- MCP `joinquest_integration_get_integration_guide`  
- PDF export (Phase C — generate from same markdown)

**Structure:**

1. Discover your game (agent interview script)  
2. Catalog voice + tag taxonomy  
3. Quick start (health, status, game-modes)  
4. Seat manifest / `seatTemplate`  
5. Provision contract + banlist  
6. JWT + claim + JWKS  
7. Launch URLs (game-minted)  
8. Testing with a private table + room invite  
9. Requesting public release  
10. Troubleshooting index mapped 1:1 to dashboard check IDs  

Dashboard error strings link to anchor IDs in this doc (`#provision-403-banned`).

---

## Phasing

### Phase A — Critical path ✅ Shipped

Registration, private testing, and the developer dashboard:

- Homepage CTA + `/developers` landing  
- Sign-in-gated registration form + draft / connect API  
- `owner_user_id`, visibility states, `myGames` query  
- Developer dashboard with full integration checklist (manifest, provision, JWT)  
- Homepage “Your games” strip for owners  
- Private testing: room tables, not public catalog  
- Post-register welcome + [integration guide](./developer-integration-guide.md)  
- **Test table** CTA (`createPrivateTable`)  

**Dependency:** [game-minted launch URLs](./game-minted-launch-urls.md) — shipped; handoff requires provision `launchUrls`.

**Optional polish:** game icons + catalog tags (see [player experience roadmap](./player-experience-roadmap.md)).

### Phase B — Go public + agents ✅ Largely shipped

- Real provision + JWT integration checks (19 required for public release)  
- **Agent-assisted catalog metadata** — discovery script, voice guide, tag taxonomy, `updateMyGameMetadata`  
- Request public release + admin review queue (`requestPublicRelease`, `reviewGameRelease`)  
- JoinQuest integration MCP + [developer-integration-guide.md](./developer-integration-guide.md)  
- Recommended local tests for game repos (integration guide §8; reference: `demo-game-rps`)

**Still open (Phase B follow-ups):**

- Scheduled re-checks + email on spec-breaking manifest changes  
- Optional `provision.banlist` remote check (games must opt in by banning test user)

### Phase C — PDF (optional / defer)

- Generate printable guide from same markdown as MCP  
- Hold until Phase B feedback settles  

---

## Open questions (minor)

- [ ] Icons required before public release, or optional v1? → **optional v1**  
- [ ] Admin review UI: in-app vs email + GraphQL script?  
- [ ] Rate limits on `run checks` / synthetic provision (per game)?  
- [ ] Webhook URL shown on dashboard for manifest-changed setup?  
- [x] Tags: fixed taxonomy vs freeform? → **fixed taxonomy v1**  

---

## Migration sketch

```sql
-- games
ALTER TABLE games ADD COLUMN owner_user_id UUID REFERENCES users(id);
ALTER TABLE games ADD COLUMN visibility TEXT NOT NULL DEFAULT 'draft'
  CHECK (visibility IN ('draft', 'private_testing', 'pending_review', 'public'));
ALTER TABLE games ADD COLUMN contact_email TEXT;
ALTER TABLE games ADD COLUMN website_url TEXT;
ALTER TABLE games ADD COLUMN community_url TEXT;

-- optional: game_integration_check_runs (game_id, check_id, status, message, detail_json, ran_at)
```

Existing catalog games: backfill `owner_user_id = NULL`, `visibility = public` (admin-owned).

---

## Success criteria

- [ ] New developer registers without DB access or admin email  
- [ ] Dashboard shows a failing manifest check with an actionable message  
- [ ] Owner creates test table; friend joins room and plays  
- [ ] Game stays off public catalog until approved  
- [ ] MCP runs full checklist; agent can fix a deliberate 403 banlist mistake from error output  
- [ ] Integration guide section IDs match dashboard deep links  
- [ ] Agent drafts catalog metadata from discovery script; developer saves via dashboard or MCP  
- [ ] Public release blocked until long description + tags present  
