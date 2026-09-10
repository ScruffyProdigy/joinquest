# Match lifecycle callbacks (game → Lobby)

Games run on their own origin after provision. Lobby still needs **lifecycle signals** so the player shell, stats, and queue rules stay correct.

**Related:** provision + `returnUrl` in [lobby-protocol-handoff.md](./lobby-protocol-handoff.md) · [player-return-routing.md](./player-return-routing.md).

---

## Two levels of reporting

| Callback | When | Purpose |
|----------|------|---------|
| **`reportPlayerFinished`** | One player is **done** with the match for themselves | Eliminated early, finished first in a race while others play, forfeited, etc. |
| **`reportMatchResult`** | The **match** is over for everyone | Winner, scores, abandoned, cancelled |

A race might call `reportPlayerFinished` for 1st, 2nd, 3rd as each crosses the line, then `reportMatchResult` when the last state is resolved.

A battle royale might call `reportPlayerFinished` on each elimination and `reportMatchResult` when one player remains (or time expires).

A synchronous 1v1 might only call `reportMatchResult` when both sides are done.

---

## Why both matter

- **Player UX** — JoinQuest can show “You placed 2nd” and enable re-queue while others are still racing.
- **Queue rules** — A player who is **finished** should not block “one queue at a time” as if they were still in an active match; Lobby clears their `matched` queue row when the game reports finish.
- **Integrity** — Final `reportMatchResult` is the authoritative outcome for leaderboards and disputes.
- **Degrades gracefully, but degrades** — a game that never calls either callback is not broken, but the player's `/return` shows only the roster: no standings, no winner, no placements. See [player-return-routing.md](./player-return-routing.md).

---

## GraphQL (game server, Bearer `serviceToken`)

```graphql
enum PlayerFinishReason {
  COMPLETED
  ELIMINATED
  FORFEIT
  DISCONNECT
}

enum MatchResultStatus {
  COMPLETED
  CANCELLED
  ABANDONED
}

mutation reportPlayerFinished(
  matchId: ID!           # Lobby externalMatchId (session id)
  lobbyUserId: ID!
  reason: PlayerFinishReason!
  placement: Int         # optional, e.g. 1 = first
  metadata: JSON
): Boolean!

mutation reportMatchResult(
  matchId: ID!
  status: MatchResultStatus!
  winnerLobbyUserIds: [ID!]
  metadata: JSON
): Boolean!
```

Lobby validates `matchId` against the game id embedded in `serviceToken`. All fields are persisted: `reportPlayerFinished` stores `reason`, `placement`, and `metadata` against that participant; `reportMatchResult` stores `status`, `winnerLobbyUserIds`, and `metadata` against the session (winner ids not seated in the match are dropped). The stored result is surfaced back to participants only — via the `matchResult` query, the `matchResultUpdated` subscription, and the standings shown on `/return` (see [player-return-routing.md](./player-return-routing.md)).

---

## Lobby behavior

1. **`reportPlayerFinished`** — mark participant finished; if all players finished, complete session; clear user’s `matched` queue row so they can join another queue.
2. **`reportMatchResult`** — mark session completed, release all `matched` queue rows for seated players.

Recommended order for games: call **`reportMatchResult`** when the match ends (clears matched queue rows), optionally **`reportPlayerFinished`** for early exits; always link players to **`{returnUrl}?match={externalMatchId}`** (see [player-return-routing.md](./player-return-routing.md)).

---

## A disconnect is not a finish

A dropped socket is not `reportPlayerFinished`. A player who closed the tab, lost
wifi, or took a phone call is still seated and still expected back — reporting them
finished releases their queue row, clears their playing intent, and permanently
closes their way back in (JoinQuest refuses to re-mint a seat token for a player who
has reported finished).

Report a player finished when they have genuinely **left** — quit, forfeited, or
timed out of a grace period you defined — not when their connection dropped.

While they are away, games should behave consistently enough that players learn one
set of expectations:

- **Hold the seat.** Don't forfeit on the first dropped socket, don't fill the seat,
  don't end the match.
- **Say so.** Show the remaining players that someone is disconnected and the game is
  waiting, rather than leaving the match silently stalled.
- **Keep moving where the design allows.** Real-time: continue and let them catch up.
  Turn-based: it is reasonable to hold on their turn with a visible timer.
- **Resync from a full snapshot** when they re-claim — complete authoritative state,
  not the deltas they missed.
- **Have an end to the grace period,** and tell the other players what it is. An
  indefinite wait is worse for the people still there than a decided outcome. *That*
  expiry is the moment to `reportPlayerFinished`.

How a player gets back in at all — the two recovery paths and the re-claim rule — is
in [lobby-protocol-handoff.md](./lobby-protocol-handoff.md#reconnecting-a-player).
