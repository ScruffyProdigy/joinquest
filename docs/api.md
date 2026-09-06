# API Documentation

GraphQL API for **JoinQuest** — the player shell and control plane for registered games.

| Client | Typical use |
|--------|-------------|
| **JoinQuest frontend** | Sign-in, browse games, `joinQueue`, subscriptions |
| **Game server** | `player(id)` with provision `serviceToken` (`displayName`, `avatarUrl`); `reportPlayerFinished`, `reportMatchResult` |
| **Admin** | `registerGame`, manifest sync |
| **Game developer** | `registerMyGame`, `myGames`, `runMyGameChecks` — see [developer-self-service.md](./developer-self-service.md) |

Platform goals and integration overview: [`vision.md`](./vision.md). Game handoff is not GraphQL — see [`lobby-protocol-handoff.md`](./lobby-protocol-handoff.md).

## Implementation Status

### ✅ Implemented
- **Authentication**: Magic-link sign-in, session cookies, JWT seat tokens
- **Catalog & matchmaking**: `registerGame`, `seatTemplate` manifest sync, mode queues, `joinQueue(queueId, queuePath)`, handoff to game servers; shareable **`gameBySlug`** detail pages — see [game-catalog-architecture.md](./game-catalog-architecture.md)
- **Database-backed queries**: `games`, `game`, `session`, `goods`, `myInventory`, `player` (game service)
- **Real-time**: `queueUpdated`, **`myTableSeatUpdated`**, and **`tableUpdated(roomId)`** subscriptions via Redis pub/sub; **`roomUpdated` / `roomMessageAdded`** for chat rooms — see [pubsub.md](./pubsub.md)
- **Post-game**: `returnDestination`, `reportPlayerFinished`, `reportMatchResult` — see [match-lifecycle-callbacks.md](./match-lifecycle-callbacks.md), [player-return-routing.md](./player-return-routing.md)
- **Rooms (Step 1)**: `createRoom`, `joinRoom`, `leaveRoom`, `sendRoomMessage`, `room`, `myRoom` — see [rooms-and-tables.md](./rooms-and-tables.md)
- **Tables (Step 2)**: forming tables, seat-level sitting, king controls — see [rooms-and-tables.md](./rooms-and-tables.md)
- **LFG forming match (Phase B)**: persistent forming map, parties, `myActiveIntent`, `formingGaps`, `startTableBackfill`, `PartyNodeInput` on `joinQueue`; **forming worker** reconciles matches asynchronously after `joinQueue` returns — [lfg-phase-b-plan.md](./lfg-phase-b-plan.md), [pubsub.md](./pubsub.md)
- **Avatars & profile**: `starterAvatars`, `updatePlayerProfile(displayName, avatarKey?)`, spirit-animal reading mutations — [spirit-animal-avatars.md](./spirit-animal-avatars.md)
- **Developer self-service (Phase A)**: `registerMyGame`, `myGames`, `myGame`, `myGameCredentials`, `runMyGameChecks`, `developerIntegrationGuide`; public `games` / `gameBySlug` filter to `visibility = public` — [developer-self-service.md](./developer-self-service.md)

### 🚧 In Development
- **Developer self-service (Phase B)**: provision/JWT integration checks (19 required), `updateMyGameMetadata`, `catalogTagTaxonomy`, `requestPublicRelease`, `reviewGameRelease`, [developer-integration-guide.md](./developer-integration-guide.md), JoinQuest integration MCP — see [developer-self-service.md](./developer-self-service.md)
- **Player-facing goods**: purchase/trade flows
- **Rate limiting**

### 📋 Planned
- **File uploads**: game asset uploads (avatars use starter assets + generated spirit-animal URLs today)
- **Phase C LFG**: weighted dequeue, `allocations` by affinity — see [seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md)
- **Scheduled integration re-checks** + email on manifest changes

## Base URL

- **Development**: `http://localhost:8080/graphql`
- **Production**: `https://joinquest.cc/graphql`

## Authentication

JoinQuest uses session cookies for browser clients (set by `completeSignInWithLink` / `completeSignInWithCode`). Include the cookie on GraphQL requests, or use `Authorization: Bearer <jwt>` where applicable.

Game servers call `player(id:)` with `Authorization: Bearer <serviceToken>` using the token from provision `lobby.serviceToken`.

## Queries

### System Queries

#### `version` ✅
Returns the current API version.

```graphql
query {
  version
}
```

**Response:**
```json
{
  "data": {
    "version": "1.0.0"
  }
}
```

#### `healthz` ✅
Health check endpoint.

```graphql
query {
  healthz
}
```

**Response:**
```json
{
  "data": {
    "healthz": "ok"
  }
}
```

### User Queries

#### `me` ✅
Get the signed-in user (null when unauthenticated).

```graphql
query {
  me {
    id
    email
    displayName
    createdAt
  }
}
```

**Response:**
```json
{
  "data": {
    "me": {
      "id": "user-123",
      "email": "user@example.com",
      "displayName": "Test User",
      "createdAt": "2024-01-01T00:00:00Z"
    }
  }
}
```

#### `player` (game service)
Resolve a lobby user by id. Requires `Authorization: Bearer <serviceToken>` from provision `lobby.serviceToken`. Used by game servers for display names and avatars in-match.

```graphql
query Player($id: ID!) {
  player(id: $id) {
    id
    displayName
    avatarUrl
    avatarSource
  }
}
```

Returns `null` if the user does not exist. Personality reading and image prompts are **not** exposed here — only the current `avatarUrl`. See [spirit-animal-avatars.md](./spirit-animal-avatars.md).

### Game Queries

#### `games` ✅
List catalog games (games with at least one active mode queue).

```graphql
query {
  games(limit: 10, offset: 0) {
    id
    name
    createdAt
  }
}
```

#### `gameBySlug` ✅
Get an active catalog game by URL slug (shareable detail page at `/games/:slug`). Returns `null` when the slug is unknown or the game is inactive. No authentication required.

```graphql
query GameBySlug($slug: String!) {
  gameBySlug(slug: $slug) {
    id
    slug
    name
    heroUrl
    catalogHeroUrl
    shortDescription
    longDescription
    howToPlay
    tutorialUrl
    screenshots
    tags
    modes {
      id
      modeKey
      displayName
      status
      queuePaths {
        queuePath
        displayName
      }
      queues {
        id
        name
        status
      }
    }
  }
}
```

**Example:** `slug: "word-hunt"` → `https://joinquest.cc/games/word-hunt`

#### `game` ✅
Get a game by ID.

```graphql
query {
  game(id: "a1000000-0000-4000-8000-000000000001") {
    id
    name
    slug
    modes {
      modeKey
      displayName
      queuePaths {
        queuePath
        displayName
        minPlayers
        maxPlayers
        playersToStart
      }
      queues {
        id
        name
        playersToStart
        waitingCount
      }
    }
  }
}
```

**`GameMode.queuePaths`** — join buckets derived from `seatTemplate` (empty for fifo / single-bucket modes). The games list UI uses `displayName` for **Join as …** labels and `playersToStart` per cohort when present.

### Session Queries

#### `session` ✅
Get a match session by ID.

```graphql
query {
  session(id: "00000000-0000-4000-8000-000000000001") {
    id
    status
    createdAt
    game {
      id
      name
    }
    players {
      id
      displayName
    }
  }
}
```

### Digital Goods Queries

#### `goods` ✅
List digital goods, optionally filtered by game.

```graphql
query {
  goods(gameId: "a1000000-0000-4000-8000-000000000001") {
    id
    code
    name
    description
  }
}
```

#### `myInventory` ✅
Get the signed-in user's inventory (requires authentication).

```graphql
query {
  myInventory {
    good {
      id
      code
      name
    }
    quantity
    grantedAt
  }
}
```

## Mutations

### Developer (owner)

#### `registerMyGame` ✅
Sign-in required. Saves a draft game, attempts API connect (`healthz` + `game-modes`), and promotes to `private_testing` on success. Returns `serviceToken` and `webhookSecret`.

#### `myGames` / `myGame` / `runMyGameChecks` ✅
Owner-scoped dashboard queries and on-demand integration checklist (19 required checks for public release: manifest, provision, JWT). Optional: `provision.banlist`.

### Catalog (admin)

#### `registerGame` ✅
Register a game from its seat manifest (`apiBaseUrl`). Requires admin (`LOBBY_ADMIN_EMAILS`).

```graphql
mutation {
  registerGame(input: {
    slug: "my-game"
    apiBaseUrl: "http://localhost:3001"
    name: "My Game"
    iconUrl: "/games/default.svg"
    heroUrl: "/games/default-hero.svg"
  }) {
    game { id slug }
    webhookSecret
  }
}
```

### Profile & avatars

#### `updatePlayerProfile` ✅
Set display name and optionally a starter avatar. **`avatarKey` is optional** — omit it to change only the name while keeping a spirit-animal or existing avatar.

```graphql
mutation {
  updatePlayerProfile(displayName: "River", avatarKey: "campfire") {
    displayName
    avatarKey
    avatarUrl
    avatarSource
  }
}
```

Spirit-animal flow mutations (`beginSpiritAnimalReading`, `submitSpiritAnimalAnswers`, `selectSpiritAnimalTotem`, etc.) are documented in [spirit-animal-avatars.md](./spirit-animal-avatars.md).

### Queue / group matchmaking

#### `myActiveIntent` ✅
The player’s current **catalog play intent** — waiting or matched in a mode queue (at most one waiting row globally). Used by the sticky intent banner.

```graphql
query {
  myActiveIntent {
    queueId
    gameId
    gameName
    status    # WAITING | MATCHED
    queuedCount
    queuePath
    queuePathDisplayName   # human label from seatTemplate (e.g. "Clue Giver")
    joinUrl   # when MATCHED
    formingGaps { queuePath displayName assigned needed }
  }
}
```

Returns `null` when not in a queue. When **WAITING** in a composition mode, `queuePath` / `queuePathDisplayName` identify which cohort bucket the player joined (banner copy uses the display name).

#### `myQueueStatus(queueId)` ✅
Status for a specific mode queue (per-game row sync).

#### `joinQueue` ✅
Start **looking for a group** in a mode queue (`queueId` from `game.modes.queues`). Requires authentication.

- **Fifo modes** (no seat `queuePath` on the mode): omit `queuePath`.
- **Composition modes**: pass the role bucket from the template (e.g. `queuePath: "ClueGiver"`). Required when the mode defines multiple queue paths.
- **Same game, different role**: if already **waiting** in that mode queue, calling `joinQueue` again with another valid `queuePath` **updates** the player's bucket (does not create a second row).
- Only one **waiting** queue per player globally. Joining another game **leaves** the previous wait list and sets **`message`** explaining the switch.
- Still **blocked** while **matched** in another queue until the player leaves or the match ends.
- **Async match:** the mutation usually returns `queued: true` immediately. When the forming worker fires a match, clients receive `joinUrl` via the **`queueUpdated`** subscription (or HTTP poll fallback). Only the player who completes the match may get `sessionId` / `joinUrl` inline on rare synchronous paths.

```graphql
mutation {
  joinQueue(queueId: "a3000000-0000-4000-8000-000000000001", queuePath: "DPS") {
    queued
    sessionId
    joinUrl
    queuedCount
    queuePath
    message
  }
}
```

**`message` example:** `You left the group for Rock Paper Scissors to look for a group here.`

#### `leaveQueue` ✅
Leave a mode queue.

```graphql
mutation {
  leaveQueue(queueId: "a3000000-0000-4000-8000-000000000001")
}
```

### Digital goods (admin)

#### `grantGood` / `revokeGood` ✅
Grant or revoke inventory for a user. Requires admin.

```graphql
mutation {
  grantGood(userId: "...", goodId: "...", quantity: 1)
}
```

## Error Handling

GraphQL returns errors in a standardized format:

```json
{
  "data": null,
  "errors": [
    {
      "message": "Game not found",
      "path": ["game"],
      "locations": [
        {
          "line": 2,
          "column": 3
        }
      ]
    }
  ]
}
```

### Common errors

GraphQL errors use plain `message` strings from resolvers, for example:

- `authentication required`
- `identity required` — the caller has a session but has not picked both a display name and an avatar. Returned by every mutation that puts a player into play (`joinQueue`, `createRoom`, `joinRoom`, `createTable`, `createPrivateTable`, `sitAtTable`, `startTable`, `startTableBackfill`). Distinct from `authentication required` so clients can raise the identity prompt rather than a sign-in flow. Leaving a queue, table or room is never gated this way.
- `game not found`
- `database store is not configured`

## Rate Limiting

Not implemented yet. See **Planned** in Implementation Status above.

## Examples

### Matchmaking workflow

1. **Sign in** (magic link or code) via `requestSignIn` / `completeSignInWithCode`.

2. **Browse catalog games**
```graphql
query {
  games(limit: 5) {
    id
    name
    modes {
      modeKey
      queues {
        id
        name
        waitingCount
      }
    }
  }
}
```

3. **Join a mode queue** (use a `queueId` from step 2)
```graphql
mutation {
  joinQueue(queueId: "a3000000-0000-4000-8000-000000000001") {
    queued
    sessionId
    joinUrl
    queuedCount
  }
}
```

4. **Watch queue updates** (WebSocket subscription; requires auth)
```graphql
subscription {
  queueUpdated(queueId: "a3000000-0000-4000-8000-000000000001") {
    status
    sessionId
    joinUrl
    queuedCount
  }
}
```

5. **Check session after match**
```graphql
query {
  session(id: "00000000-0000-4000-8000-000000000001") {
    id
    status
    createdAt
  }
}
```
