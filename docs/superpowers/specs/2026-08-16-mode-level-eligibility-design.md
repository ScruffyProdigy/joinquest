# Mode-level eligibility — design

**Status:** Draft
**Author:** Ryan Kohler (with Claude)
**Date:** 2026-08-16
**Tickets:** [JQ-11](https://linear.app/joinquest/issue/JQ-11), [JQ-12](https://linear.app/joinquest/issue/JQ-12), [JQ-13](https://linear.app/joinquest/issue/JQ-13), [JQ-16](https://linear.app/joinquest/issue/JQ-16)

## Problem

Today `GameMode` has no concept of per-player eligibility — every active mode is joinable by every player. Four related tickets need a whole-mode lock/unlock mechanism:

- **JQ-11** — a mode can be gated behind completing another mode (e.g. a tutorial). Boolean gate, no progress to show. Previously an open design question ("does JoinQuest need this for MVP?"); resolved 2026-08-16 as yes (see Decision Log).
- **JQ-12** — a mode can be gated behind a single counter threshold (e.g. 50 ranked wins for a "Legendary" mode), with visible progress (current/target).
- **JQ-13** — a mode can be gated behind a boolean player-state check owned by the game (e.g. "has at least one Standard-legal deck"), routing the player to a different mode (Deck Builder) when ineligible.
- **JQ-16** — a mode can be gated behind a **compound** requirement — two or more counters combined with AND/OR (e.g. 25 Standard wins AND 5 unique decks used).

All four tickets explicitly call for "the same mode-level eligibility mechanism" — this spec designs that one mechanism, not four separate features.

**Note on the reference tickets' concrete examples:** the ticket descriptions cite a Figma Make prototype (`src/app/data/games.ts`, games "Iron Fist Arena" and "Cardstone") as the source of the exact field shapes and scenarios. That prototype does not exist anywhere in this repository (confirmed across all branches) — it's an external design artifact. The only real game integrations in this repo are the reference games in [reference-games.md](../../reference-games.md) (Rock Paper Scissors Lizard Robot, Word Hunt), neither of which has these mode concepts. Per decision during brainstorming: this work builds the **generic eligibility mechanism** as a platform capability, demonstrated end-to-end with a **fixture game** rather than by modifying a real reference game or Iron Fist Arena/Cardstone (which aren't buildable targets in this repo).

## Goal

A generic, per-player, per-mode eligibility mechanism, following the existing "game owns the logic, JoinQuest just displays/enforces" pattern already established in [composition-and-join-options.md](../../composition-and-join-options.md) for within-mode choices — extended here to whether an entire `GameMode` is joinable at all.

## Non-goals

- Not modifying `docs/composition-and-join-options.md`'s existing within-mode queue-option mechanism (JQ-15's territory) — this is a separate, whole-mode gate, cross-linked but not merged with that doc.
- Not building or wiring this into a real third-party reference game — fixtures only (see Scope decision above).
- Not persisting eligibility state in JoinQuest's own database — the game server is the source of truth; JoinQuest never stores stats.
- Not supporting eligibility gates on anything other than `GameMode` (e.g. no queue-path-level or seat-level gating) — that's the existing, separate within-mode mechanism.

## Architecture & data flow

1. Game servers **optionally** implement `GET /api/v1/players/{lobbyUserId}/mode-eligibility`, returning eligibility for every mode of that game in one payload (not one call per mode).
2. A new `backend/internal/gameclient` method calls this endpoint per-player, with a short in-memory TTL cache (~5s) keyed by `(gameID, lobbyUserID)` — same "cache briefly, never as source of truth" posture the docs already prescribe for queue-options.
3. If a game server doesn't implement the endpoint (404/timeout/malformed response), every mode for that game **fails open**: `accessible: true`. This keeps the feature purely opt-in — existing reference games are unaffected without any changes on their end.
4. `GameMode` gets a new GraphQL field, `eligibility(playerId: ID!): ModeEligibility`, resolved via the gameclient method above.
5. The frontend queries this alongside the existing mode list (no extra round trip) and renders locked state + progress + routing.

## Data model

**GraphQL** (new, added to `catalog.graphqls`):

```graphql
type ModeEligibility {
  accessible: Boolean!
  reason: String
  requirement: ModeRequirementNode
  unlockModeKey: String
}

interface ModeRequirementNode {
  label: String!
}

type RequirementLeaf implements ModeRequirementNode {
  label: String!
  current: Int!
  target: Int!
}

type RequirementGroup implements ModeRequirementNode {
  label: String!
  operator: RequirementOperator!
  children: [ModeRequirementNode!]!
}

enum RequirementOperator {
  ALL
  ANY
}

extend type GameMode {
  eligibility(playerId: ID!): ModeEligibility
}
```

`requirement` is `null` for pure boolean gates with nothing countable to show (JQ-11's tutorial-complete gate — just `accessible`/`reason`). When there's progress to show, `requirement` is populated: a bare `RequirementLeaf` for a single counter (JQ-12), a `RequirementLeaf` with `target: 1` for a boolean has/has-not check (JQ-13's "has a Standard-legal deck"), or a `RequirementGroup{operator: ALL, children: [...]}` for a compound gate (JQ-16). The leaf/group interface is recursive, so nested groups are supported without further API changes even though no current ticket needs nesting beyond one level.

`unlockModeKey` is optional and populated only when the game server has a specific actionable next step to point the player at — e.g. JQ-13 sets it to the Deck Builder mode's key so the frontend can route there on click. `null` when there's no single obvious next mode (JQ-12's "go win more ranked matches" isn't a mode to route to).

`RequirementGroup.operator` in GraphQL is the enum `ALL`/`ANY`; the game server's JSON contract uses lowercase `"all"`/`"any"` (matching the rest of the JSON contract's lowercase `kind` field) — the Go deserializer uppercases it when mapping into the GraphQL enum type. Any other value is a malformed response and triggers the same fail-open handling as a 404.

**Go** (`backend/internal/gameclient/eligibility.go`, new): a mirrored tagged-union type deserializing the same leaf/group JSON shape from the game server's HTTP response, mapped 1:1 into the generated GraphQL resolver types. No persistence — this is transport only.

## Game-server HTTP contract

```
GET /api/v1/players/{lobbyUserId}/mode-eligibility
```
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

A compound gate's `requirement`:

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

A mode key absent from the response, or the whole call failing, falls back to `accessible: true` (fail-open, per the architecture section above).

## Fixture game and gate mapping

One fixture game (seeded similarly to the existing `000003_demo_games` migration), with five modes — four demonstrating a gate, plus an always-accessible Deck Builder mode as the routing target for JQ-13:

| Ticket | Mode | Gate shape |
|---|---|---|
| JQ-11 | Tutorial-gated main mode | `accessible`/`reason` only, no `requirement` |
| JQ-12 | Legendary | `RequirementLeaf` — ranked wins current/target |
| JQ-13 | Standard | `RequirementLeaf` — has-a-deck boolean (`target: 1`), `unlockModeKey` set to `"deck-builder"` |
| JQ-16 | Commander | `RequirementGroup{ALL}` of two `RequirementLeaf`s |
| — | Deck Builder | Always `accessible: true` — exists only as JQ-13's routing target |

The fixture game server's canned responses are keyed by test player ID (e.g. `player-legendary-locked` / `player-legendary-unlocked`) so both states are exercised without mutating real stats. Implemented as:
- An `httptest`-backed handler under `backend/internal/gameclient/testutil`, reused by Go integration tests.
- A thin standalone binary (`backend/cmd/fixturegame` or similar) wrapping the same handler, runnable locally so the frontend UI can be visually verified against real GraphQL responses.

## Frontend

`ModeRow` (`frontend/src/components/games/GameModesPanel.jsx`) gets a new branch at the top: when `mode.eligibility?.accessible === false`, render a locked variant in place of the normal `GameQueueActions`/create-private-table controls, showing:
- `reason` text.
- If `requirement` is present, a recursive progress readout: a `RequirementLeaf` renders as `current/target label`; a `RequirementGroup` renders its `children` joined by "and"/"or" per `operator`.

Clicking the locked row's reason routes to `unlockModeKey` when present (JQ-13's "route to Deck Builder"), reusing the panel's existing mode-switch navigation; when absent, the row is informational only (no click target). Locked modes are shown-but-disabled, not filtered out of the list.

Existing join/queue/private-table logic is untouched, gated entirely behind the `accessible` check.

**Known overlap:** branch `jq-40-derive-gamemode-min-max` has unmerged changes to the same file (`isSoloMode()` instant-play branch in `ModeRow`) and to `frontend/src/lib/games.js`. Both changes are additive branches on different fields (`isSoloMode()` vs. `eligibility.accessible`) — expected to merge cleanly, but flag again before landing either branch.

## Testing

- **Go unit tests** on the new gameclient eligibility method: JSON parsing, TTL cache hit/miss, fail-open behavior on 404/timeout/malformed response.
- **Go integration tests** (following the existing `backend/graph/*_integration_test.go` pattern): spin up the fixture game server, exercise the `GameMode.eligibility` resolver end-to-end for all four gates, both locked and unlocked.
- **Frontend component tests**: `ModeRow` locked-state rendering for leaf, boolean-leaf, and group cases; routing-on-click behavior.
- **Manual/visual verification**: run the fixture binary + frontend dev server, confirm all four locked states render and unlock correctly in the browser before calling this done.

## Rollout

Single PR/branch covering JQ-11, JQ-12, JQ-13, and JQ-16 together (they share one mechanism and were already consolidated into this branch/worktree). No feature flag — opt-in by construction (fail-open when a game doesn't implement the endpoint), so nothing behaves differently for existing games until the fixture game (or a real game, later) implements the contract.
