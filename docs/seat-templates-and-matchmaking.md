# Seat templates, seat map, and LFG

JoinQuest matchmaking fills a **seat map** before your game sees a roster — so authors publish layout (`seatTemplate`) and we handle parties, gaps, and queue fairness. That keeps game code simple and lets one platform serve many titles. See [`vision.md`](./vision.md).

**Audiences**

| Reader | Read this |
|--------|-----------|
| **Game site / agent authoring manifests** | [Contract for games](#contract-for-game-sites), [Manifest](#mode-object), [Cookbook](#cookbook), [Game checklist](#checklist-game-sites) |
| **Lobby implementers** | [What Lobby builds](#what-lobby-builds), [Working seat map](#working-seat-map), [LFG engine](#lfg-engine), [Appendix](#appendix-lobby-implementation) |
| **Product / future agents** | [Canonical scenarios](#canonical-lfg-scenarios), entire doc |

**Related:** provision + JWT in [`lobby-protocol-handoff.md`](./lobby-protocol-handoff.md); catalog overview in [`game-catalog-architecture.md`](./game-catalog-architecture.md).

### Implementation status (Lobby code)

**Shipped:** `seatTemplate` required on manifest sync (flat `seats[]` rejected); expansion to
`game_mode_seats` with `queue_path`; `joinQueue(queueId, queuePath)` and path-aware
matchmaking (fifo within each path); JoinQuest **Look for group** / **Join as …** UI from
expanded paths; **`affinity_key` derivation** at expand time (Phase B step 1).

**Shipped (Phase B):** forming match persistence, parties, `PartyNodeInput` tree — see
[lfg-phase-b-plan.md](./lfg-phase-b-plan.md).

**Not yet (Phase C):** weighted dequeue, `allocations` by affinity, player relocation on the forming map,
multiple concurrent forming matches. Sections marked **Phase C target** describe future engine behavior.

---

## What Lobby builds

Lobby is a **seat map LFG engine**:

1. Games publish a **`seatTemplate`** — the legal layout of seats (teams, roles, slots).
2. Lobby **expands** it once into a **seat map**: every `seatKey` the game will ever see for that mode.
3. While players queue, Lobby maintains a **working seat map** for the **current forming match** — assigning and **moving** people to satisfy **party constraints** and fill **gaps**.
4. When the map is full and valid, Lobby **fires**: provision push + per-player JWTs use the **same** `seatKey` assignments.
5. Waiters not on that map stay in the **queue** for the **next** match (not rejected).

**Games do not run matchmaking.** They only accept the **final seat map** (who sits in which `seatKey`). The template exists so Lobby knows **how** seats relate (same team, same role bucket, capacity) and can rearrange waiters before send.

```text
seatTemplate (game manifest)  →  expand at sync  →  full seat map (structure)
                                                      ↓
Queue + party constraints  →  LFG engine  →  working map (forming match)
                                                      ↓
Fire  →  seat map + userIds  →  game provision + JWT seatKey
```

---

## Contract for game sites

### Your only layout obligation

After Lobby fires a match, you receive a **seat map**: a list of assignments:

```json
{ "seatKey": "Team-1-Seat-2", "lobbyUserId": "…", "team": "1", "role": null }
```

Each player’s launch JWT includes the **same** `seatKey` (see [`lobby-protocol-handoff.md`](./lobby-protocol-handoff.md)).

| Do | Don’t |
|----|--------|
| Bind game state to `seatKey` from provision/JWT | Parse team structure from string prefixes (fragile) |
| Reject tokens whose `seatKey` you did not assign | Run your own queue or re-seat players |
| Publish `seatTemplate` in `/api/v1/game-modes` | Publish flat `seats[]` (legacy, rejected) |
| Treat `seatKey` as an opaque stable id per mode | Hardcode `p1`/`p2` unless they match expansion |

You **do not** implement LFG, party logic, or queue paths. You **do** need to handle every `seatKey` your template expands to.

### What you publish

- **`seatTemplate`** — required; defines the map.
- **`min` / `max` / `sizeForQueue`** — only when the mode supports variable match sizes (see [Mode sizing](#mode-sizing)).
- Optional **`count`** — validation only; must equal expanded leaf count if present.

Lobby derives player UI (**Look for group** vs **Join as …** role buckets) from the template; you do not configure button labels in the manifest.

---

## Mode object

```json
{
  "key": "sixes",
  "displayName": "3v3",
  "seatTemplate": {
    "Team": { "count": 2, "Seat": { "count": 3 } }
  }
}
```

| Field | Required | Purpose |
|-------|----------|---------|
| `key` | yes | Mode id (`gameMode` on provision) |
| `displayName` | yes | Lobby UI |
| `seatTemplate` | yes | Layout tree → seat map. **All sizing lives here** — `min`, `max`, `sizeForQueue` are keys inside `seatTemplate` (at the root for a single-path mode, or per named dimension for composition modes), never sibling fields on the mode object. |

**Rejected:** `seats[]`, top-level `min`/`max`/`minPlayers`/`maxPlayers`/`sizeForQueue`, `count` on the mode, `rosters[]`, `matchmaking.kind`, `queue`, `startPolicy`, `assignment`.

### Mode sizing

| Layout | Author sends | LFG fire size | Whole-mode min/max |
|--------|--------------|---------------|---------------------|
| Fixed (duel, 3v3, full role comp) | `seatTemplate` only | **Derived** = number of expanded leaves | **Derived** = leaf count |
| Variable (e.g. 2–5 players) | `min`, `max`, optional `sizeForQueue` *inside* `seatTemplate` | `sizeForQueue` or **`max`** | Sum of each path's own `min`/`max` (falls back to that path's full seat count if it doesn't declare one) |

Do **not** duplicate fixed size as `count` on the mode — Lobby derives it from the template.

---

## `seatTemplate` grammar

### Keys

- **PascalCase** — dimension kinds (`Team`, `Seat`, `DPS`, `Offense`, …). Game-defined.
- **lowercase** — reserved only: `count`, `name`, `displayName`, `min`, `max`, `sizePolicy` (**deferred** — do not use in v1).

`{}` on a node = one instance for expansion, but affects **naming** (below).

### Seat key expansion

Each **leaf** becomes one `seatKey`: path segments joined with `-`.

| Authored | Segment |
|----------|---------|
| `{}` | `Kind` only — **no** `-1` |
| `{ "count": 1 }` | `Kind-1` |
| `{ "count": N }`, N > 1 | `Kind-1` … `Kind-N` |
| `{ "name": ["Red","Blue"] }` | `Kind-Red`, `Kind-Blue` |
| Root `{ "count": 4 }` | `1`, `2`, `3`, `4` |

**Order:** parent instances in order, then children in stable PascalCase key order → `sort_order` at sync.

**Lobby also derives per leaf:**

| Field | Use |
|-------|-----|
| `affinity_key` | Side bucket for “same team” (e.g. `Team:1`) — from template path |
| `queue_path` | Which lobby queue this seat belongs to (see below) |

**Examples**

```text
Support: {}     → Team-1-Support
Support: {count: 1} → Team-1-Support-1
3v3             → Team-1-Seat-1 … Team-2-Seat-3
```

### Queue paths (lobby UI only)

Walk the template; register a queue at each [branch endpoint](#queue-path-algorithm). Games do not see `queuePath`.

| Paths | UI |
|-------|-----|
| 1 (`""`) | **Look for group** (single button; fifo; `joinQueue` in API) |
| 2+ | **Look for group** panel with **Join as …** per path (composition) |

**Algorithm**

```text
walk(node, path):
  if ≥2 PascalCase children: recurse into each child with path + [child]
  if 1 child:              recurse without extending path
  if 0 children:           register queue at path (leaf-bearing node)
```

**Collapse example:** `Defense: { Warrior: { count: 2 } }` → one queue **`Defense`**, not `Defense.Warrior`.

---

## Working seat map

### States

| State | Meaning |
|-------|---------|
| **Empty map** | Forming match created from template; no users assigned |
| **Filling** | Some cells assigned; gaps remain |
| **Ready** | `assignedCount == sizeForQueue` and all constraints valid |
| **Fired** | Committed to session; game provision sent; **new** forming match started if waiters remain |

**v1:** one forming match per mode at a time.

### Cell assignment

```text
seatKey → empty | lobbyUserId
```

Only players **on this forming match** count toward fire. Everyone else stays in the **waiter queue** for the same mode (or composition path).

**Important:** A party that **cannot** fit the current forming map (e.g. third “team of 2” in 3v3) is **not rejected** — they remain queued for the **next** match after the current one fires.

---

## Party constraints (not “everyone picks a seat”)

> **Phase C target.** Phase B uses **party layout trees** (`PartyNodeInput` / `party_tree`) aligned with
> `seatTemplate` branches, plus **pinned** table seats for backfill. The `together` / `allocations`
> JSON shapes below are the planned Phase C API — not the current GraphQL surface.

Parties express **constraints on the assignment problem**. They do **not** need to pick final `seatKey`s unless they use **pins**.

### Constraint types (v1)

**1. Together** — all members on the same `affinity_key`:

```json
{ "together": true, "size": 2 }
```

Lobby may place on Team 1 or Team 2 and may **move** the pair to Team 2 if that helps fill the map (unless pinned).

**2. Split allocations** — members divided across sides (e.g. 2+1 in 3v3):

```json
{
  "allocations": [
    { "affinity": "Team:1", "size": 2 },
    { "affinity": "Team:2", "size": 1 }
  ]
}
```

Lobby fills **gaps**: needs +1 on Team 1 and +2 on Team 2 from other waiters (solos or complementary parties).

**3. Pins** (optional) — hard hold on specific `seatKey`s; no rearrangement off those cells:

```json
{ "together": true, "size": 2, "pins": ["Team-1-Seat-1", "Team-1-Seat-2"] }
```

Use pins sparingly. Prefer **together** or **allocations** so Lobby can slide groups (e.g. second pair from Team 1 seats to Team 2).

**4. Composition queue path** — which role bucket they joined:

```json
{ "queuePath": "DPS" }
```

### What we are not doing (v1)

- Requiring every player to pick a `seatKey` at join (“seat pick overload”).
- `sizePolicy` / Codenames-style asymmetric team sizes.
- Partial role-composition scaling (6-player Overwatch on 10-seat template) — composition modes use **full** template fill.

---

## LFG engine

LFG is **not** “strict FIFO until N players.” It is **constraint-based seat map filling**.

### Loop (conceptual)

```text
on join(solo or party):
  attach to waiter queue (and composition path if applicable)
  try to place on current forming match if constraints fit
  if placed: update gaps; else: wait (still in queue for this or next match)

repeatedly:
  compute gap vector per affinity / queuePath
  select waiters to add (see dequeue policy)
  assign or relocate on working map without breaking pins
  if map full and valid: FIRE

on FIRE:
  commit seat map → session → provision + JWTs
  start new empty forming match
  continue with remaining waiters (e.g. third pair waiting for game 2)
```

### Gap filling

After partial placement, Lobby tracks **needs** per affinity (and per `queuePath` in composition):

```text
need[Team:1] = capacity(Team:1) - assigned(Team:1)
```

Pull solos or parties that **reduce gaps**. Ideal case: two parties with **complementary** 2+1 splits fill each other with **no** solos.

### Dequeue policy (not strict FIFO)

When pulling from the waiter queue, Lobby uses a **score**, not queue head only:

| Factor | Why |
|--------|-----|
| **Complementarity** | Party fills current gaps (e.g. second 2+1 split) |
| **Party size** | Larger groups are harder to place later — favor slightly over long-waiting solos when re-solving the map (see [fifo stall example](./lfg-phase-b-plan.md#example-count-3-with-a-b-solo-then-cd-party)) |
| **Skill / MMR** | When tracking win rates, bias toward even-skill lobbies in the same dequeue pass |
| **Already on forming map** | Prefer finishing partial placements (Phase B); may yield to size/fit in Phase C when relocation is enabled |
| **Wait time** | Tie-break for fairness within same tier |

```text
score(party, formingMap) = wFit * fitBonus(gaps) + wSize * party.size + wAge * ageSeconds
```

Exact weights are implementation constants; the spec requires **fit-first, then size, then age**.

### Fire condition

| Mode | Fire when |
|------|-----------|
| Fixed layout | `assignedCount == derived leaf count` (and map valid) |
| Variable fifo | `assignedCount == sizeForQueue` (≤ template leaves) |
| Composition v1 | All `queuePath` buckets at full template counts (no partial scaling) |

### Placement at fire

1. **Pinned** cells fixed.
2. **Together** / **allocations** satisfied.
3. Any remaining cells: **uniform random** among valid empties (solos).
4. Same `seatKey`s → provision + JWT.

### Start now (outside LFG)

Party leader **`startMatch(modeId, party, startSize)`** — immediate match at chosen size in `[min, max]` without waiting for `sizeForQueue`. Lobby builds a one-off map (same constraint rules).

---

## Canonical LFG scenarios

### A. Three “teams of two” in 3v3

Template: 2 teams × 3 seats. Three parties, each `{ together, size: 2 }`, many claim “Team 1 seats 1–2” as a **preference** (soft unless pinned).

**Forming match 1**

| Step | Map |
|------|-----|
| Party 1 | `Team-1-Seat-1`, `Team-1-Seat-2` |
| Party 2 | Moved to `Team-2-Seat-2`, `Team-2-Seat-3` (same team constraint, team 1 full) |
| Gaps | +1 on Team 1 (`Seat-3`), +1 on Team 2 (`Seat-1`) |
| Fill | Two **solos** from queue |
| Fire | 6 players → game 1 |

**Party 3** — still in queue; **waiting for game 2** (not rejected). At most **one** intact pair per team on a 6-player map.

### B. One party 2+1 in 3v3

Party of 3: `{ allocations: [{ affinity: Team:1, size: 2 }, { affinity: Team:2, size: 1 }] }`.

| Side | Have | Need from queue |
|------|------|-----------------|
| Team 1 | 2 | +1 |
| Team 2 | 1 | +2 |

Lobby pulls waiters (solos or small groups) into those gaps until the map is full.

### C. Two complementary 2+1 parties

| Party | Team 1 | Team 2 |
|-------|--------|--------|
| P1 | 2 | 1 |
| P2 | 1 | 2 |

Combined → 3v3 per side, **no solos**. High `fitBonus` — should be matched ahead of unrelated fifo waiters.

---

## Composition modes (role queues)

When the template has **multiple queue paths** (e.g. DPS / Tank / Support):

- Players join **one** path.
- LFG fills **per-path** needs from full template expansion.
- **Split allocations** and **together** groups still apply **within** the forming map; paths add “which bucket you queued as.”

**v1:** variable `sizeForQueue` + partial role counts are **out of scope** — use fixed full templates for composition.

---

## Agent workflow: human → manifest

| Human says | `seatTemplate` | Also on mode |
|------------|----------------|--------------|
| 1v1 / duel | `{ "count": 2 }` | — |
| FFA N | `{ "count": N }` | — |
| 3v3 | `Team: { count: 2, Seat: { count: 3 } }` | — |
| Role pick (Overwatch) | `Team` + DPS/Tank/Support | — |
| Nested roles | `Offense`/`Defense` trees | — |
| 2–5 players, LFG at 4 | max 5 leaves | `min`, `max`, `sizeForQueue: 4` |
| 2–5 players, LFG at max | max 5 leaves | `min`, `max` |

---

## Cookbook

### 1v1

```json
{
  "key": "duel",
  "displayName": "Duel",
  "seatTemplate": { "count": 2 }
}
```

### 3v3

```json
{
  "key": "sixes",
  "displayName": "3v3",
  "seatTemplate": {
    "Team": { "count": 2, "Seat": { "count": 3 } }
  }
}
```

### Overwatch-style (composition)

```json
{
  "key": "quick-play",
  "displayName": "Quick Play",
  "seatTemplate": {
    "Team": {
      "count": 2,
      "DPS": { "count": 2 },
      "Tank": { "count": 2 },
      "Support": {}
    }
  }
}
```

### Offense / Defense (composition)

```json
{
  "key": "siege",
  "displayName": "Siege",
  "seatTemplate": {
    "Offense": { "Wizard": {}, "Warrior": { "count": 2 } },
    "Defense": { "Warrior": { "count": 2 } }
  }
}
```

### Variable fifo

```json
{
  "key": "casual",
  "displayName": "Casual",
  "seatTemplate": { "count": 5, "min": 2, "max": 5, "sizeForQueue": 4 }
}
```

`min`/`max`/`sizeForQueue` are keys on the `seatTemplate` node itself, not siblings of it. Omit `sizeForQueue` to default LFG to `max`.

### Word Hunt Party (composition + per-path sizing)

```json
{
  "key": "party",
  "displayName": "Word Hunt Party",
  "seatTemplate": {
    "ClueGiver": { "displayName": "Clue Giver", "name": ["Red", "Blue", "Green"], "min": 2, "max": 3, "sizeForQueue": 2 },
    "Guesser": { "count": 6, "min": 2, "max": 6, "sizeForQueue": 4 }
  }
}
```

No top-level `min`/`max` needed — Lobby derives the mode's whole-table bounds (4–9 players) by summing each path's own `min`/`max`, and fires LFG at 6 (2 ClueGivers + 4 Guessers) from each path's `sizeForQueue`.

---

## Checklist: game sites

- [ ] `seatTemplate` expands to every seat the game can spawn
- [ ] Game binds players only to provision/JWT `seatKey`
- [ ] No flat `seats[]`
- [ ] `{}` vs `{ "count": 1 }` tested for intended names (`Team-1-Support` vs `Team-1-Support-1`)
- [ ] Integration test: manifest → expected `seatKey` list

---

## Checklist: agents (Lobby + manifest)

- [ ] Queue path walk matches intended lobby buttons
- [ ] Fixed layout: no `min`/`max`/`sizeForQueue` unless variable
- [ ] `min <= sizeForQueue <= max` when variable
- [ ] Party model uses `together` / `allocations` / `pins` — not mandatory seat picks
- [ ] Understand third pair **waits for next match**, not rejected
- [ ] `displayName` omitted when same as kind name

---

## Deferred (not v1)

- `sizePolicy` (Codenames 3v4, sandbagging)
- `rosters[]` / multiple LFG profiles per mode (use `sizeForQueue` or separate modes)
- Proportional partial composition (6 on 10-seat template)
- Cross-path parties (DPS + Tank in one party)
- Multiple concurrent forming matches per mode

---

## Appendix: Lobby implementation

### Core tables (target)

| Entity | Purpose |
|--------|---------|
| `game_modes.seat_template` | Raw tree |
| `game_mode_seats` | Expanded leaves: `seat_key`, `affinity_key`, `queue_path`, `sort_order` |
| `forming_match` | Current map: `mode_id`, `target_size`, state |
| `forming_assignment` | `forming_match_id`, `user_id`, `seat_key`, `party_id`, `pinned` |
| `game_queues` | Waiters; link to `mode_queue_id`, optional `forming_match_id` when placed |

### GraphQL (shipped)

```graphql
input PartyNodeInput {
  role: String
  children: [PartyNodeInput!]
  members: [PartyMemberInput!]
}

joinQueue(queueId: ID!, queuePath: String, party: PartyNodeInput): JoinResult!
leaveQueue(queueId: ID!): Boolean!
startTableBackfill(tableId: ID!, queueId: ID!): JoinResult!

type GameMode {
  derivedCount: Int!
  min: Int!
  max: Int!
  sizeForQueue: Int!
  matchmakingKind: MatchmakingKind!
  seats: [GameModeSeat!]!
  queues: [ModeQueue!]!
  forming: FormingMatchStatus
}
```

### Implementation phases

| Phase | Deliverable | Status |
|-------|-------------|--------|
| A | Template expander + sync + game manifest validation | **Shipped** |
| B | Working seat map + parties + split `queuePath` + fire → provision/JWT | **Shipped** — see [lfg-phase-b-plan.md](./lfg-phase-b-plan.md) |
| C | `allocations` + gap scoring + weighted dequeue | Planned |
| D | Composition paths + coordinator | Planned |
| E | `sizeForQueue` variable fifo | Planned |

### Migration

Legacy flat `seats[]` → `seatTemplate` only. Manifest `schemaVersion: 1` recommended.

---

## Quick reference

| Topic | Rule |
|-------|------|
| Game contract | Final `seatKey` → `lobbyUserId` map only |
| Template purpose | Structure for Lobby moves + expansion |
| LFG | Fill forming map with constraint + gap heuristic |
| Third pair in 3v3 | Waits for **next** match |
| 2+1 party | `allocations`; pull +1 / +2 from queue |
| Two complementary 2+1 | Prefer pairing in dequeue score |
| `{}` vs `{count:1}` | `Kind` vs `Kind-1` |
| Variable size | `min`, `max`, `sizeForQueue` (default `max`) |
