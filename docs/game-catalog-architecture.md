# Game catalog, modes, and queues

Locked architecture decisions for **third-party game integration** on JoinQuest.

**Why this matters for game authors:** Players discover your title in our catalog and queue here; we fill seats and hand off to your game. You publish modes and a seat layout (and optionally **launch URL bases** on provision — see [game-minted-launch-urls.md](./game-minted-launch-urls.md)); we own matchmaking. Product story: [`vision.md`](./vision.md). Wire protocol: [`lobby-protocol-handoff.md`](./lobby-protocol-handoff.md).

## Model

```text
Game
  └── GameMode (many)
        ├── seatTemplate (from cached game-modes manifest; expanded to leaf seats)
        └── Queue (many)
              ├── single path (`""`): one “Look for group” bucket (e.g. 1v1 duel)
              └── composition: one bucket per `queuePath` (DPS, Tank, …)
```

- **Game** — `slug`, `api_base_url`, sync metadata.
- **GameMode** — one way to play; identified by `mode_key` sent on provision.
  A two-player mode is a mode with two seats, not a special “duel” type.
- **Queue** — matchmaking bucket scoped to `queue_id` (fifo default or role-specific).
- **Session** — `game_id`, `mode_id`, optional `queue_id` (null for table/room starts).
- **Room / Table** — social room (invite code) + forming table per game/mode; see [rooms-and-tables.md](./rooms-and-tables.md).

**Full spec (game authors + Lobby LFG):** [`seat-templates-and-matchmaking.md`](./seat-templates-and-matchmaking.md) —
`seatTemplate` → seat map (game contract); constraint-based LFG fills the map before provision.

## Seat fill (shipped vs Phase C target)

**Shipped (Phase A + B):** games publish **`seatTemplate` only** (flat `seats[]` is rejected on sync).
Lobby expands the template to leaf seats (`seat_key`, `queue_path`, `affinity_key`, `sort_order`),
stores `seat_template` on the mode, and auto-creates one default queue per mode.

**Catalog LFG:** solo **Look for group** / **Join as …** joins a **persistent forming match** per
mode queue; players are placed incrementally on the working seat map; **`formingGaps`** show
remaining role needs on the intent banner.

**Table backfill:** friends sit at a table → king **Look for group** → same forming match fills
from catalog. **`startTableBackfill`** + gap visibility on `TableCard`.

**Phase C (target):** weighted dequeue, `allocations` by affinity, optional relocation of players
already on the map. Games still receive final `seatKey` assignments only. See
[seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md).

## Catalog card artwork

Each game registered in the catalog must supply **`iconUrl`** and **`heroUrl`** (see `RegisterGameInput`). These are separate assets with different aspect ratios and placements.

| Field | Aspect ratio | Where it appears |
|-------|----------------|------------------|
| **`heroUrl`** | **16∶9** on the dedicated game page; **5∶2** on the catalog card when no `catalogHeroUrl` | Detail page hero banner; catalog card fallback |
| **`catalogHeroUrl`** | **5∶2** (optional) | Catalog card banner when set; otherwise `heroUrl` is used |
| **`iconUrl`** | **1∶1** (square) | Small thumbnail on room **table cards** (not on the catalog hero) |

### Dedicated game page (`/games/:slug`)

Each catalog game with a `slug` has a shareable detail page. From the catalog, the **whole card** links to this page.

- **Hero:** **`heroUrl`** in a **16∶9** slot (`aspect-ratio: 16/9`, `object-fit: cover`). Recommended exports: 960×540, 1280×720, 1920×1080.
- **Description:** **`longDescription`** (GraphQL; stored in `games.description`) when present, otherwise **`shortDescription`**.
- **Optional sections:** **`howToPlay`**, **`tutorialUrl`** (external link), **`screenshots`** (image URL array).
- **Play:** signed-in users get mode/queue actions here — the catalog card itself has no queue actions, it's a pure navigation tile (no section heading on the detail page).
- **Navigation:** the whole catalog card links here; **Back** restores catalog scroll position; **Share** copies or opens the page URL.

Query: `gameBySlug(slug: String!)` on the GraphQL API.

### Hero banner (`heroUrl` / `catalogHeroUrl`)

- **Catalog aspect ratio:** **5∶2** — e.g. 400×160, 1000×400, 1200×480, 1600×640.
- **Detail page aspect ratio:** **16∶9** — e.g. 960×540, 1280×720.
- **Layout:** The catalog card reserves a **5∶2** slot (`aspect-ratio: 5/2` in CSS) and renders the image with **`object-fit: cover`**. The detail page uses **16∶9**. Artwork is cropped to fill the slot; important content should stay in a safe center band.
- **Formats:** JPEG, PNG, WebP, or SVG. Production demo games use **JPEG** heroes (e.g. `/games/word-hunt-hero.jpg`).
- **Hosting:** Static path under Lobby’s `/games/` prefix or any HTTPS URL. Assets in `frontend/public/games/` are whitelisted in `.gitignore` and served by the frontend nginx (see `frontend/nginx.conf`). Game artwork URLs get a `?v=` cache-bust query param in the frontend (`GAME_ARTWORK_VERSION` in `gameCard.js`).
- **Reference assets:** Placeholder SVGs use a **400×160** viewBox (`word-hunt-hero.svg`, `default-hero.svg`, etc.); shipped heroes are full-color JPEGs in the same directory.

### Icon (`iconUrl`)

- **Aspect ratio:** **1∶1** (square).
- **Layout:** Shown at **64×64** on room table cards with **`object-fit: cover`**.
- **Recommended export:** **256×256** or **512×512** PNG (or SVG).

### Other catalog copy

- **`shortDescription`** — one-line blurb under the hero on the catalog card.
- **`longDescription`** — full blurb on the dedicated game page (falls back to `shortDescription`).
- **`howToPlay`**, **`tutorialUrl`**, **`screenshots`** — optional detail-page content.
- **`tags`** — the first two tags are shown as a combined badge pill on the catalog card (see `gameGenreModeLabel` in the frontend); the detail page shows up to three as separate chips (see `gameTagChips`).

## Admin registration

- Admin calls `registerGame` (email allowlist via `LOBBY_ADMIN_EMAILS`).
- Lobby immediately pulls `{apiBaseUrl}/healthz`, `/api/v1/status`, `/api/v1/game-modes`.
- **Fail closed** — do not list the game if health or game-modes fetch fails.
- Cache normalized modes/seats plus `manifest_hash`, `manifest_etag`, `manifest_synced_at`.
- Issue a per-game **webhook secret** for manifest-change callbacks.

## Manifest updates

- **Primary:** game `POST`s lobby `…/games/{slug}/manifest-changed` (authenticated with webhook secret).
- **Backup:** periodic poll of `/api/v1/game-modes` (ETag when supported).
- **In-flight matches** — never changed by a manifest refresh.
- **Added mode** — list in catalog; auto-create one default queue (`players_to_start = len(seats)`).
- **Removed mode** — disable mode and its queues.
- **Removed queue** — **kick all waiting players** with a user-visible message (e.g. via
  `queueUpdated` with status `LEFT` and reason text); do not start new matches on that queue.
- **Seat key changes** that break existing queues — disable affected queues and kick waiters with a message until an admin re-enables or recreates queues.

## Queues

- **Hybrid creation:** on register, auto-create queue(s) from template (default fifo, or
  one row per `queue: true` role dimension).
- Matchmaking is **scoped to `queue_id`**, not `game_id`.
- **One waiting queue per player** globally (migration `000012`). Joining another queue **automatically leaves** the previous waiting queue; the client shows a `message` on `joinQueue`.
- **Fifo:** FIFO within one queue until `players_to_start` (default `max`).
- **Composition:** match fires when every role bucket reaches `required`; coordinator
  assigns teams (see seat-templates doc).

## URLs

- **`api_base_url`**: Game API **origin** only (e.g. `http://localhost:3001`, `https://rpsls-duel.win`) — same value as JWT `aud`. Lobby posts to `{api_base_url}/api/v1/matches`. Do not include the `/api` ingress prefix here.
- **Launch URLs**: Returned on provision (`launchUrls` or `launchUrlTemplate`); Lobby attaches the seat JWT. There is no catalog `play_url` column.

## Provision

- `lobbyId` = `LobbyIssuer()`; `lobby.returnUrl` = browser Lobby URL; `lobby.graphqlUrl` = `{issuer}/graphql`; `lobby.serviceToken` = per-game credential `v1.{gameUUID}.{hmac}` from `LOBBY_GAME_TOKEN_PEPPER` (also returned by admin `registerGame.serviceToken`). Games store it per match; provision POST uses `Authorization: Bearer` with the same value. Legacy global `LOBBY_GAME_SERVICE_TOKEN` is dev-only fallback.
- `gameMode` = mode’s `mode_key` (game server resolves mode-specific rules locally).
- Pass `team` / `role` on seats from the cached template when applicable.
- Player display names via `player(id)` at `lobby.graphqlUrl` using `lobby.serviceToken` from provision.
- Banlist handshake (`403` + `bannedLobbyUserIds`) unchanged.

## Player API (target)

```graphql
games {
  modes {
    composition { slotKind required waitingCount }
    queues { id name slotKind requiredTotal playersToStart waitingCount }
    seats { seatKey affinityKey slotKind }
  }
}
joinQueue(queueId: ID!, queuePath: String, party: PartyNodeInput): JoinResult!
queueUpdated(queueId: ID!): QueueUpdate!  # include message/reason when kicked
```

Details in [`seat-templates-and-matchmaking.md`](./seat-templates-and-matchmaking.md).

Non-queue start paths (party, direct) are **v2**; schema keeps `queue_id` nullable on sessions.

## Admin auth

- `LOBBY_ADMIN_EMAILS` comma-separated allowlist; signed-in session required.
- `me { isAdmin }` for client gating; phone demos use normal sign-in, not API keys.

## Environments

- One catalog row per environment (e.g. joinquest.cc vs local), each with its own URLs and cached manifest.

## Deferred

- Signed provision requests (allowlist `lobbyId` + shared Bearer today).
- Webhook when manifest changes.
- Populate `team` / `role` on provision seats from manifest.
- Phase C LFG: weighted dequeue, `allocations` by affinity ([seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md)).
