# Rooms and tables

Social **rooms** for friends to gather; **tables** are forming private games inside a room.

> **Superseded on design by the prototype (2026-09-06).** The Figma Make prototype at
> [demo.joinquest.cc](https://demo.joinquest.cc) is the newer standard, and it replaces the
> two-level room→table model with a single per-mode **"Your Group"** screen, reached from a
> **Play with friends** action beside **Jump in** on each mode row. That screen shows seat
> occupancy ("4 of 4 seats · ready to start" / "Still need: Player"), a roster, per-seat claiming,
> and a **Share Link** — with no chat, no invite code, no QR, no king, and nothing persisting
> past the match.
>
> The intent is **not** to delete the room model. Rooms and tables remain the substrate; the group
> screen is a simpler presentation over them, so chat, invite codes, QR, the king role and
> multi-table rooms can be surfaced again as the UX matures rather than rebuilt. Tracked in JQ-131
> (entry point) and JQ-132 (backing it with an implicit room + table).
>
> Note one place production is arguably ahead: the prototype has only a share link, while
> production also has QR — the better mechanic for players in the same room, and being kept.
>
> The rest of this document remains accurate as the description of the shipped model.

**Related:** [seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md) · [game-catalog-architecture.md](./game-catalog-architecture.md) · [player-return-routing.md](./player-return-routing.md) · [composition-and-join-options.md](./composition-and-join-options.md)

---

## Roadmap

| Step | Scope |
|------|--------|
| **Step 1 — Rooms** (shipped) | Chat rooms: create/join, short invite code, member list, **QR** + easy link sharing. One room per player. |
| **Step 2 — Tables** (shipped) | 0..N forming tables per room; **seat-level sitting** (`sitAtTable(tableId, seatKey)`); king controls; queue ↔ table mutual exclusion; per-mode catalog actions; lazy stale discard. |
| **Step 3 — Table LFG** (shipped) | King **Look for group** backfill; `formingGaps` on TableCard; catalog fills remaining roles. |
| **Step 4+** | Phase C: affinity-aware weighted dequeue. |

---

## Step 1: Rooms (chat + invite)

See git history / prior docs for Step 1 detail. Rooms remain the social shell: invite code, chat, share/QR, one room per player globally.

**Step 2 change:** sitting at a table or joining a queue are **mutually exclusive** (see below). Chat-only room membership still works alongside either intent.

---

## Step 2: Tables (forming games)

### Concepts

| Entity | Purpose |
|--------|---------|
| **Table** | One **forming** match for a **game + mode** inside a room. Many tables per room (unbounded). |
| **Seat** | A specific expanded `seatKey` from the mode template (not a queue path alone). |
| **King** | Longest-sitting player (`min(seated_at)`). Recomputed when anyone leaves. Shown in UI (“King: Pat”). Controls **Start now** and **Discard** when eligible. |

```text
Room (invite code, chat)
  └── Table × N (forming)
        ├── game + mode
        ├── seat-level sitting (seatKey)
        ├── king: Start now; **Look for group** backfill (Phase B)
        └── on Start → session + provision (return to room)
```

### Player intent (global)

| Slot | Rule |
|------|------|
| **Table seat** | One active table seat globally. Sitting **leaves** any other table seat. |
| **Queue vs table** | Mutually exclusive — `sitAtTable` clears waiting queue; `joinQueue` clears table seat. |
| **Matched queue** | Cannot sit at a table until the active match is left. |

### Sitting and caps

- Join via **`sitAtTable(tableId, seatKey)`** — `seatKey` must exist on the mode. For **pooled fifo groups** (shared `queuePath`, or flat duel seats like RPSLS `1`/`2`), the key identifies **which group** to join; the server assigns the **next open seat** in that group in template order. Stale clients that send an already-taken seat still succeed. For **non-pooled** layouts (pick-your-own-seat roles), the exact seat must be empty.
- Errors: **`store: no open seats in this group`** when that role bucket is full; **`store: table is full`** when `maxPlayers` is reached; **`store: seat is already taken`** only for non-pooled exact-seat picks.
- **Start** requires **king** (earliest `seated_at`), total ∈ `[minPlayers, maxPlayers]`, and every template path meets its **minimum** (path `min`, else `playersToStart` for that path). Only the king sees **Start now**.
- **Team layout in UI** uses `seatKey` prefix (e.g. `Team-1` vs `Team-2`). Role caps come from shared queue paths (e.g. Overwatch `DPS` cap 4 across both teams). **`affinityKey` is not used in Step 2.**

### Catalog UX

Per **mode** row under each game:

- **Look for group** — existing queue join (unchanged).
- **Play with friends** — `createPrivateTable(gameId, modeId)` creates a room if needed, sweeps stale empty tables, adds a forming table, then navigates to `/group`. The room is never presented as a step of its own: the player asked to play with friends, not to make a room.
  - A visitor with no name and avatar is rejected by the backend with `identity required`, which raises the identity picker. The intent is held in `sessionStorage` keyed by game + mode (`lib/pendingGroupIntent.js`) and resumed on `onAuthComplete`, so picking a name continues into the group instead of dropping them back on the catalog.

### Group screen (`/group`)

A **presentation over one table in a room** — not a second data model. It renders entirely
from `Table.seatSlots`, `Table.formingGaps`, `Table.king` and `Room.members`, and adds no
GraphQL operations of its own.

- **Which table:** the one the player is seated at, else the newest forming table in their
  room (`selectGroupTable`). A room holding several tables keeps the room surfaces instead.
- **Sections:** accent header with live status (`Still need: <roles>` → `N of M seats · ready
  to start`), **Invite friends** (QR + share link, reusing `RoomShareToolbar`; the raw invite
  code is hidden here), **Players** (seat rows with Claim/Leave), **Picking a seat**
  (`Room.members` minus the seated players), and a sticky bottom control.
- **Bottom control:** `Claim a seat to join` when unseated; `Start game` for the king once
  `canStart`; otherwise `Waiting for <name> to start`. The king gate is production's, but the
  screen never uses the word "king" — it names the person.
- **Leaving is navigating away.** Presence on the page is the seat: a deliberate in-app
  navigation calls `leaveTable`, and the last seated player out also calls `discardTable`
  (`leaveTable` only deletes the seat, so the emptied table would otherwise linger). There is
  deliberately **no `beforeunload` handler** — a reload, a closed tab or a dropped connection
  keep the seat.
- **Share links:** `/room/:CODE` joins the room, then lands on `/group` when the room holds
  exactly one forming table. Arrivals land **unseated**, in *Picking a seat*, and claim their
  own seat rather than being placed in a role they did not choose.
- **What is hidden, not removed:** chat, the invite code, the king role and multiple tables
  per room all remain in the schema, resolvers and the room surfaces. `/group` suppresses the
  desktop room panel and the mobile dock so the room does not show through beside it.

### Table UI (room panel)

- **Tables (N)** section above chat.
- **TableCard:** game/mode, king badge, team columns when `Team-N` prefixes appear, individual seat chips or a single **Sit** button per pooled group, king-only **Start now**, **Look for group** backfill (when seated), role **formingGaps**, **Discard** when eligible.
- **Intent banner:** `myActiveIntent` (catalog wait / matched) or `myTableSeat` (table forming / backfill).
- **Realtime fallback:** while any forming table is open, the room panel polls every 3s so `canStart` and seat occupancy stay fresh when the `tableUpdated` WebSocket is delayed.

### Stale tables

- Discardable when **zero seated** and (**≥ 60s old** OR king manually discards).
- Auto-sweep on **create table** removes stale empty tables first.
- No countdown UI; button label **Discard**.

### Return routing

```json
{
  "kind": "room",
  "path": "/room/X7K2M9",
  "gameId": "…",
  "roomId": "…",
  "tableId": "…"
}
```

### Data model

#### `room_tables`

| Column | Notes |
|--------|--------|
| `id`, `room_id`, `game_id`, `mode_id` | Many per room |
| `status` | `forming`, `started`, `discarded` |
| `session_id` | Set on start |

#### `table_seats`

| Column | Notes |
|--------|--------|
| `table_id`, `user_id` | **Unique `user_id`** globally |
| `seat_key` | Expanded template key |
| `seated_at` | King tie-break |

### GraphQL (Step 2)

```graphql
type Table {
  id: ID!
  game: Game!
  mode: GameMode!
  createdAt: Time!
  # PublicPlayer, not User: a table admits strangers via Look for group and the
  # catalog queue, so nothing on the card carries an email.
  king: PublicPlayer
  seats: [TableSeat!]!
  seatSlots: [TableSeatSlot!]!
  canStart: Boolean!
  canDiscard: Boolean!
  lookForGroupOptions: [TableLookForGroupOption!]!
}

extend type Room { tables: [Table!]! }

extend type Query { myTableSeat: MyTableSeat }

extend type Mutation {
  createPrivateTable(gameId: ID!, modeId: ID!): Table!
  createTable(roomId: ID!, gameId: ID!, modeId: ID!): Table!
  sitAtTable(tableId: ID!, seatKey: String!): Table!
  leaveTable(tableId: ID!): Boolean!
  discardTable(tableId: ID!): Boolean!
  startTable(tableId: ID!): JoinResult!
}

extend type Subscription {
  tableUpdated(roomId: ID!): Table!
}
```

Realtime: Redis `lobby:room:{roomId}` publishes `table_updated` events (same channel as chat/membership). See [pubsub.md](./pubsub.md) for Redis setup and debug tracing.

### Template validation

Catalog sync rejects `seatTemplate` when two queue paths share the same **`displayName`** (case-insensitive). Paths are **not** merged in the UI.

---

## Locked product defaults

| Decision | Choice |
|----------|--------|
| Build order | Rooms first, tables second |
| Tables per room | 0..N |
| Sitting | **Seat-level** (`seatKey`) |
| King | Longest-sitting player; visible |
| Tables per player | 1 seat globally |
| Queue vs table | Mutually exclusive |
| Private game | No queue; binds game + mode only |
| Table LFG | **Look for group** backfill (Phase B) |
| Stale empty tables | Lazy sweep + manual Discard |
| Catalog | Per-mode LFG + Play with friends |
| Group screen | `/group` — one table, presented without the room |
