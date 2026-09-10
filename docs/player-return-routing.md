# Player return routing

Games send players to a **stable return hub** after a match. Lobby resolves **where each player goes next** from context stored at match time.

**Related:** [lobby-protocol-handoff.md](./lobby-protocol-handoff.md) · [match-lifecycle-callbacks.md](./match-lifecycle-callbacks.md)

---

## Contract (games)

| Field | Value |
|-------|--------|
| `lobby.returnUrl` on provision | `{LOBBY_PUBLIC_URL}/return` (e.g. `https://joinquest.cc/return`) |
| Player exit link | `{returnUrl}?match={externalMatchId}` |

Games store `returnUrl` from provision once per match. When a player leaves, link them to the URL above. Lobby’s `/return` page loads signed-in, reads `returnDestination(matchId)`, and redirects.

**Do not** embed per-player paths in the game — Lobby owns routing.

---

## Return context (Lobby DB)

When a player is seated for a match, Lobby stores JSON on `game_session_participants.return_context`:

```json
{
  "kind": "catalog_lfg",
  "path": "/",
  "gameId": "…",
  "modeQueueId": "…"
}
```

| `kind` | Meaning | Typical `path` |
|--------|---------|----------------|
| `catalog_lfg` | Joined via catalog queue | `/` (home / catalog) |
| `room` | Room after table start | `/room/{inviteCode}` |

Room/table spec: [rooms-and-tables.md](./rooms-and-tables.md).

New entry paths set context at join/match time. The return hub only reads it.

---

## GraphQL

**Player (session cookie):**

```graphql
query ReturnDestination($matchId: ID) {
  returnDestination(matchId: $matchId) {
    path
    kind
  }
}
```

**Game server (Bearer `serviceToken` from provision):** see [match-lifecycle-callbacks.md](./match-lifecycle-callbacks.md).

---

## Environment

| Variable | Role |
|----------|------|
| `LOBBY_PUBLIC_URL` | Browser origin (e.g. `https://joinquest.cc`) |
| `LOBBY_RETURN_URL` | Optional full return hub URL override (must include `/return` path if set) |

If unset, return hub = `LOBBY_PUBLIC_URL` + `/return`.

---

## Building the player exit link

Games receive `lobby.returnUrl` (the hub base) on provision. Append the Lobby session id (`externalMatchId`) when linking players back:

```
{returnUrl}?match={externalMatchId}
```

**Reference implementations** (keep in sync):

| Language | Location |
|----------|----------|
| Go | `backend/internal/returnlink.AppendMatchID` (also `auth.LobbyReturnURLForMatch`) |
| TypeScript | `demo-game-rps/client/src/lobbyReturn.ts` → `buildLobbyReturnLink` |

The JoinQuest frontend does not build this link — only games do. The `/return` hub resolves routing via `returnDestination`.

---

## Implementation status

- Return hub route: `/return` (frontend) — resolves `returnDestination`, and when a `matchResult` exists for that match and viewer, shows a results view (standings once the game has reported, "still playing" roster otherwise) plus a regroup offer, instead of redirecting straight through
- `returnDestination` query
- `return_context` on session participants (`catalog_lfg` at match create)
- `reportPlayerFinished` / `reportMatchResult` mutations — persist the reported outcome (see [match-lifecycle-callbacks.md](./match-lifecycle-callbacks.md))
- `matchResult(matchId: ID!)` query and `matchResultUpdated(matchId: ID!)` subscription — the stored outcome (participants, placements, winner, each player's regroup state), visible only to players who were in that match
- `playAgain` / `declinePlayAgain` mutations — the "who's playing again?" regroup, reconstituting one shared table from the finished roster

### Seating on rejoin

Whether a returning player is put back in their old seat turns on one question: does the
mode ask them to choose anything at all? `store.ModeOffersPreMatchChoice` answers it —
more than one seat class (`seattemplate.Leaf.NamePath`, the same interchangeable-seats
fact the rating layer keys on, so N identical player seats are one class), or any
pre-queue option group. `GameMode.hasPreMatchChoice` exposes the same answer to clients.

| | Nothing to choose | Something to choose |
| -- | -- | -- |
| Match completes (`resetRoomTableAfterSessionTx`) | everyone re-seated on their old seat | table comes back **empty** |
| `playAgain`, group play | seated | **not seated** — they choose again on `/group` |
| `playAgain`, solo play | seated | seated, replaying **their** seat and pre-queue options |

The solo and the group rule are different by design. A solo player almost always wants
exactly what they just had; a group came back to rotate the spymaster or bring a different
character, and pre-selecting last round's pick pre-empts that as firmly as pre-seating
pre-empts rotation. Tests assert both so neither is later "fixed" into the other.

*Solo* is `MatchResult.groupPlay`, read from the viewer's own `return_context.kind`: it is
stamped when the session starts and never rewritten, unlike `room_tables.session_id`
(cleared by the reset) and `game_sessions.regroup_table_id` (stamped by whichever player
claims first). Per viewer, not per match — a stranger backfilled into a group's table did
queue alone.

A group claimant is therefore `IN` while holding no seat. That is "returned, still
picking", which is what `/group`'s *Picking a seat* card renders; `canStart` is gated on
seats and never on the opt-in stamp.

On the return screen this gives a solo player a second action, *Choose again*, wherever
`mode.hasPreMatchChoice` — the way to reach those choices rather than inherit them. It
goes back to the game page, where the mode's picker lives, and takes the place of
*Back to \<game\>* rather than sitting beside it.

**The `{returnUrl}?match={externalMatchId}` contract above is unchanged and carries no result data.** That URL is player-editable, so it can never be trusted to carry an outcome — the authoritative path for match results is the server-to-server `reportPlayerFinished` / `reportMatchResult` callbacks the game already makes. `/return` looks up the result itself from `matchId`; it never reads one off the query string.
