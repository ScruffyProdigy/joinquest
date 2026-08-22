# Composition queues and “join as” options

This clarifies **composition matchmaking** (“Join as DPS / Tank”) vs a **character-select screen** inside the game.

**Seat / queue spec:** [seat-templates-and-matchmaking.md](./seat-templates-and-matchmaking.md)

---

## What “Join as” means in JoinQuest (today / near term)

For **composition** modes, the template defines **queue paths** (e.g. `Offense.Wizard`, `Defense`). The player picks a **role bucket** before entering matchmaking — not necessarily a specific character skin.

That is **lobby-side queue selection**, similar to picking a role in a MOBA queue. It is **not** a full character-select UI with animations and loadouts.

For **single-bucket** modes (simple 1v1 / FFA — one `queuePath`, usually `""`), the JoinQuest UI shows a single **“Look for group”** button; omit `queuePath` on `joinQueue`.

For **composition** modes, role choices appear **inside a “Look for group” panel** — e.g. “Join as Clue Giver”, “Join as Guesser” — so the player is still “looking for a group”, just in a specific role bucket.

While waiting, the player can **switch roles** in the same game by clicking another **Join as …** button; Lobby updates their `queue_path` without leaving the queue. The sticky banner shows **`queuePathDisplayName`** (from `seatTemplate`) so the active cohort is always visible.

---

## GraphQL surface (catalog UI)

The games list reads **`GameMode.queuePaths`** — one entry per join bucket with `queuePath`, `displayName`, `minPlayers`, `maxPlayers`, and per-cohort **`playersToStart`** (from `sizeForQueue` when set).

```graphql
query {
  game(id: "…") {
    modes {
      queuePaths {
        queuePath
        displayName
        playersToStart
      }
    }
  }
}
```

**Fifo modes** return an empty `queuePaths` list; the UI shows a single **Look for group** control inside the same panel chrome.

**`myActiveIntent`** returns `queuePath`, `queuePathDisplayName`, and `formingGaps` while **WAITING** so the banner can say e.g. “Looking for a group in Word Hunt as Clue Giver · Need 1 Clue Giver”.

See [api.md](./api.md) for `joinQueue` path switching and subscription fields.

---

## What belongs on the game site

Many games need **eligibility** that Lobby cannot guess from a static manifest:

- Which classes or decks this **account** may play
- **DLC** or season pass gates
- **Progression unlocks** (“after 10 games, unlock Advanced class”)
- Balance rules (“no hard support in solo queue”)

Those should come from the **game server**, not from a fixed list in `seatTemplate`.

### Recommended pattern (target)

1. Player chooses mode on JoinQuest (and role bucket if composition).
2. Before or during join, Lobby calls the game (authenticated as that player), e.g.  
   `GET /api/v1/players/{lobbyUserId}/queue-options?modeKey=…&queuePath=…`
3. Game returns allowed options: `{ "choices": [{ "id": "wizard", "label": "Wizard", "locked": false }, …] }`
4. JoinQuest UI shows only allowed choices; `joinQueue` sends the selected `queuePath`. Optional `party: PartyNodeInput` tree for API/tests; friends use table backfill instead.
5. After matchmaking, provision still sends **`seatKey`** — the game maps role → concrete character in-client if needed.

Lobby caches responses **briefly** (seconds), not as source of truth. DLC and unlock changes stay on the game.

**Related but separate:** `queue-options` narrows role choices *within* a mode you can already join. A related, separate mechanism — **mode-level eligibility** — gates whether an entire `GameMode` is joinable at all (tutorial-complete flags, win-count thresholds, compound requirements). See [§12 of the developer integration guide](./developer-integration-guide.md#12-mode-level-eligibility-optional) for the endpoint contract.

---

## Why not only a manifest?

`seatTemplate` describes **layout and affinity** (teams, slots, capacities). It does not know player inventory, MMR brackets, or unlock progress. Duplicating unlock logic in the manifest would drift from the game and break DLC/progression models.

---

## Character select screen?

| Screen | Where | When |
|--------|-------|------|
| Role / queue bucket | JoinQuest | Before queue, drives matchmaking |
| Eligibility list | JoinQuest (data from game API) | Before queue, optional step |
| Cosmetic / loadout / map character | **Game client** | After JWT claim, before or during match |

So: **not** replacing the game’s character select — **optional** pre-queue eligibility on JoinQuest, fed by the game.

---

## v1 scope

- **Shipped:** `seatTemplate` manifest sync; one default queue per mode; **Look for group** UI;
  **`GameMode.queuePaths`** with display names; composition modes show **Join as …** from that metadata;
  in-queue **role switching**; sticky banner cohort label; `joinQueue(queueId, queuePath)`
  and path-aware matchmaking with per-path **`sizeForQueue`** fire thresholds.
- **Deferred (Phase C+):** weighted dequeue, `allocations` by affinity, game `queue-options` API, DLC-aware polling,
  unlock progression.
- **Rooms & tables:** Step 1 = chat **rooms** (invite, QR, share); Step 2 = **tables** for forming games — [rooms-and-tables.md](./rooms-and-tables.md).

For new composition demos, start with **static paths from `seatTemplate`**; add **game-backed options**
before production titles with progression.
