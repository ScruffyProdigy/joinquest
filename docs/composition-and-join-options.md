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
4. JoinQuest UI shows the choices, **including locked ones with their unlock requirement** (see the prototype note below — this reverses the earlier "only show unlocked" guidance); `joinQueue` sends the selected `queuePath`. Optional `party: PartyNodeInput` tree for API/tests; friends use table backfill instead.
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
- **Deferred (Phase C+):** weighted dequeue, `allocations` by affinity, DLC-aware polling.
- **Now MVP (2026-09-06):** pre-queue options as a platform capability, including game-reported
  locked choices and progression unlocks. Previously listed as Phase C+ here; superseded — see JQ-163.
  Only *purchase-derived* entitlements (which need the lobby to own entitlement data) remain deferred.
- **Rooms & tables:** Step 1 = chat **rooms** (invite, QR, share); Step 2 = **tables** for forming games — [rooms-and-tables.md](./rooms-and-tables.md).

For new composition demos, start with **static paths from `seatTemplate`**; add **game-backed options**
before production titles with progression.

---

## Prototype model (2026-09-06) — newer standard

The Figma Make prototype at [demo.joinquest.cc](https://demo.joinquest.cc) is the newer standard;
where this document disagrees with it on design, the prototype wins. Each mode there carries:

```js
{
  key, name, default, blurb,
  minPlayers, maxPlayers,
  typicalMinutes,        // duration, per mode (2, 5, 12, 45 …)
  socialMode,            // Free-for-all | 1v1 | Teams | Hidden roles | Co-op
  roleSelect,            // null, or "Crew (4–6) · Saboteur (1)"
  preQueue,              // null, or { kind, label, options, locking }
  partyFit,              // Stays together | Solo only | May be split | Can fill a side
  access,                // Open | Locked
  unlockRequirement,     // "Win 5 Casual matches"
  composition            // FIFO | Seat template
}
```

Four things this settles that the sections above got wrong or left open:

1. **Selection order is mode → role → options.** Which options exist can depend on the mode, so a
   mode must be chosen first. `preQueue` is a property of the mode.
2. **Options are mode-scoped, not role-scoped.** Picking a role does not narrow the option roster.
   (In the built prototype the mode's roster is *copied onto every role* and read back through the
   selected one, so the plumbing for role-scoped rosters exists — but every role carries the same
   list, which is what makes the behaviour mode-scoped. Shipped as mode-scoped; see JQ-163.)
3. **Locked options are shown with a reason**, not hidden. The prototype renders
   `lockedReason: "Unlock by playing more matches"`. Step 4 above has been corrected accordingly.
4. **Unlocks are usually not purchases.** Every unlock condition in the prototype is progression —
   "Finish one Settlement game", "Reach account level 12", "Win 5 Casual matches". Option locking
   therefore has no dependency on payments, which is why it is MVP scope.

In the prototype's catalog data `preQueue` is `{ kind, label, options, locking }`, where `kind` is
`Loadout`, `Character` or `Deck`, `label` is the player-facing prompt ("Choose your champion"), and
`locking` is `none` or `some` — `some` meaning part of the roster is locked for this player, not
that the mode is.

## Shipped shape (JQ-163)

Production splits that summary in two, because a roster that can include "the decks you built" can
never live in a manifest:

- **The mode's manifest declares the groups only** — `preQueue: { groups: [{ key, kind, label, min,
  max }] }`. `min`/`max` generalise the prototype's implicit pick-one (RPSLR asks for two helpers);
  `min: 0` makes a group optional.
- **The game serves the choices per player** at
  `GET {apiBaseUrl}/api/v1/players/{lobbyUserId}/queue-options?modeKey=…`, each with `locked` and
  the same requirement tree mode-eligibility uses.

Unlike mode-eligibility this does **not** fail open — a mode whose roster cannot be loaded becomes
unjoinable and says so, because neither an empty nor a permissive guess is safe. A `preQueue` block
the lobby cannot parse is refused the same way (JQ-211): dropping the picker does not disable the
mode, it provisions the match with an empty selection and tells nobody, which is the one outcome
worth being loud about. Manifest sync rejects a bad declaration on the way in, so a row that fails
this check predates that gate and wants a developer.

Selections travel on `joinQueue` and on `sitAtTable` (each player answers for themselves as they
claim a seat), are validated server-side against the roster the player was actually served, and
reach the game as `assignment.seats[].options`.

Validation is in two halves, because the two questions have different lifetimes. `prequeue.Validate`
runs at pick time and is the only place the roster is consulted — whether an option exists and
whether it is locked are facts only the game knows, about one player, at one moment.
`prequeue.ValidateDeclared` asks the declaration alone (which groups exist, how many picks each
takes, which are required) and so can run anywhere, including inside a transaction. The store calls
it at the seat claim, at a table backfill, and at `addSessionParticipantTx` — the one place both
provision paths meet — so no seat reaches a game with a selection its mode would not accept, by any
path. Re-checking the roster there instead would fail a started match because somebody's unlocks
moved while they waited.

Contract details: [§13 of the developer integration guide](./developer-integration-guide.md#13-pre-queue-options-optional).
