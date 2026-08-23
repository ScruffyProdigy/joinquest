---
name: multiplayer-game-design
description: >-
  Design and architecture guidance for games with more than one real player -- real-time or
  turn-based, competitive or cooperative. Use when deciding what must be server-authoritative
  vs. client-only, handling player disconnect/reconnect, designing turn structure, downtime, or
  pacing, balancing competitive fairness (turn order and first-player advantage, catch-up
  mechanics, runaway-leader and rubber-banding problems, kingmaking, symmetric or asymmetric
  balance), protecting hidden information from client-side extraction (card hands, fog of war),
  reasoning about latency fairness in real-time games, or avoiding cooperative-game pitfalls
  like quarterbacking. Draws on both video-game networking practice and tabletop/board-game
  design theory. Not for single-scene code structure or chat/communication systems (EventBus,
  GameState, layering) -- see game-architecture for that.
---

# Multiplayer Game Design

Guidance for the design and architecture questions that only exist once a game has more than one real player. `game-architecture` covers how a game's code is structured (including chat/communication systems); this skill covers what changes once other people are in the match with you -- drawn from both multiplayer networking practice and tabletop/board-game design theory, since the underlying problems (fairness, hidden information, what happens when someone leaves) are older than video games and well-studied outside them.

A useful frame for any of the sections below: mechanics (the rules you write) produce dynamics (what actually happens when people play), which produce the aesthetics (what it feels like to play) -- the MDA framework. A rule that looks fine on paper (a strong early lead, a "pause" button, a fully-visible game state) can produce a dynamic that ruins the aesthetic (a snowballing runaway leader, a stalled match, a solved game). Most of what follows is a catalogue of rules that look reasonable but produce bad dynamics in a multiplayer context specifically.

## Client / Server Authority

Every multiplayer game splits responsibility between a client (rendering, input, local feel) and a server (the shared source of truth). Getting this split wrong is the single most common architecture mistake in multiplayer games -- most bugs, cheating vectors, and "worked in testing, broke with real players" incidents trace back to state living in the wrong place.

### What must be server-authoritative

Anything that affects other players, determines the outcome of the match, or must survive a client disconnecting needs to live on the server, not just be reported by the client:

- Match/round state (whose turn, current score, win/loss/draw determination)
- Anything used for matchmaking fairness or anti-cheat (position for hit detection, resource counts, timers that gate actions)
- The result that gets recorded/persisted after the match ends

A simple test: if a malicious or buggy client could send a fabricated value for this and affect another player's experience, it must be validated or computed server-side, not trusted from the client.

### What's safely client-only

- Rendering, animation, particle effects, camera work
- Local input handling and prediction (moving your own character smoothly before server confirmation)
- Purely cosmetic state (which idle animation is playing, local settings like volume)

### Common failure modes

- **Trusting client-reported outcomes** -- client says "I won," server just believes it. Fine for a solo game, a real bug in anything competitive or cooperative.
- **State that only exists in one client's memory** -- if the server never learns about it, it can't be part of a reconnect resync (see Session Continuity below) and other players have no way to know about it.
- **Recomputing authoritative values from client input without validation** -- e.g. accepting a client-sent "damage dealt" number instead of the server independently computing it from the actual game rules.

This split is also what makes the disconnect/reconnect pattern below possible at all -- you can only resync a reconnecting player from a source of truth that isn't the thing that just disconnected.

### A Note on Latency Fairness

Server authority solves cheating (a client can't fabricate state), but it doesn't automatically solve latency fairness -- when two players' actions reach the server with different network delay, the server still has to pick whose reality wins. This only matters for fast-paced, precision-timing real-time games, where "who acted first" or "was the target still there" decides an outcome down to fractions of a second: competitive shooters popularized "favor the shooter" lag compensation, where the server rewinds its state to check a shooter's hit against what the shooter actually saw, which can feel fair to the shooter and feel deeply unfair to whoever got hit from behind cover on their own screen. For slower-paced real-time games, turn-based games, or anything where a few hundred milliseconds of variance wouldn't change who made the better decision, this mostly doesn't apply -- don't add lag-compensation complexity unless the genre's core mechanic actually depends on sub-second timing between players.

## Multiplayer Session Continuity (Disconnect / Reconnect)

For any match-based multiplayer game, this is the design question that actually matters in place of pausing: **what happens when a player's connection drops or their tab loses focus mid-match?** Players close laptops, switch tabs, and lose wifi far more often than they intentionally quit -- the game needs a policy for this, not just a happy path. Design for it from the start; retrofitting reconnection onto a game that never planned for it is much harder than building it in from the beginning.

### Principles

1. **Server-authoritative state is required for reconnect to work at all.** If match state lives only in each client's memory, a disconnected client has nothing to resync from when it comes back.
2. **Detect disconnect explicitly** -- WebSocket `close`, `beforeunload`, and `visibilitychange` (tab backgrounded) are different signals with different intent; a backgrounded tab on mobile is not the same as an intentional quit, so don't treat them identically.
3. **Grace period before removing a player from the match** -- give a disconnected player a window (e.g. 15-30s, tune per game pace) during which their seat is reserved and the match continues without them, rather than instantly forfeiting or ending the match. Communicate this state to remaining players ("Waiting for Player 2 to reconnect...") rather than leaving it silent.
4. **Reconnect resyncs from a full server snapshot, not deltas** -- when a player reclaims their seat, send the complete authoritative state, not an incremental patch. The client should never have to guess what it missed. Persist enough of the player's own state (position, hand, inventory, whatever applies) that the resync is exact, not approximate.
5. **Define the grace-period expiry behavior per game** -- options include: forfeit the disconnected player, hand them to an AI/bot for the remainder, or end the match for everyone with a clear reason shown to remaining players. Pick one deliberately; don't leave it undefined and let the client silently hang.
6. **Reconnect UI is part of the core loop, not polish** -- a `Reconnecting...` state with a visible timer belongs in the same tier as the win/lose screen, not bolted on afterward. On reconnect, route the player straight back into the match they left rather than back to a menu.

### Event additions for this pattern

```js
export const Events = {
  // ...existing events...

  // Session continuity
  PLAYER_DISCONNECTED: 'session:player_disconnected',
  PLAYER_RECONNECTED: 'session:player_reconnected',
  GRACE_PERIOD_EXPIRED: 'session:grace_period_expired',
  STATE_RESYNCED: 'session:state_resynced',
};
```

## Turn Structure: Real-Time vs. Turn-Based vs. Asynchronous

These are different architectural problems, not points on a difficulty scale -- pick based on the game, not by default:

- **Real-time (everyone moving simultaneously)** needs client-side prediction (act locally before server confirmation) plus server reconciliation (correct the client when the server's authoritative result differs) to feel responsive over real-world latency. For frame-precise competitive genres (fighting games, precise platformer duels), rollback netcode (predict remote input, roll back and resimulate on misprediction) is the genre-standard approach and worth adopting wholesale rather than reinventing -- but it's overkill for slower-paced or turn-based games.
- **Turn-based (players act in sequence)** is architecturally simpler: a reducer/command pattern works well -- server and every client run the same pure `(state, event) -> newState` function over an ordered event log, so keeping clients in sync is a matter of making sure everyone has the same event log, not the same continuously-interpolated position. Pair with a per-turn timer and idle-timeout policy (auto-skip, auto-forfeit after N missed turns) instead of letting a slow player stall the match indefinitely.
- **Asynchronous (players act minutes/hours/days apart, can go offline between turns)** adds a notification problem on top of turn-based sync -- players need to be told it's their turn out-of-band (push notification, badge, email), and the UI needs to gracefully show "waiting on Alex" as a normal, non-error state rather than something to be anxious about.

### Downtime and Pacing

Whichever structure you pick, decide deliberately what an idle player experiences -- "nothing to look at" reads as broken, not as a pause:

- **Turn-based waiting**: show the board/opponent's state if the game has open information, or at minimum a countdown and whose turn it is if it doesn't. Allow low-stakes actions during someone else's turn (chat, emotes, reviewing your own hand -- see `game-architecture`'s Communication Systems section for implementing chat/emotes) that can't affect the outcome, rather than locking the whole UI. Tune the turn timer to the game's actual decision complexity -- a fast-reaction game and a deep-strategy game need very different timers, and it's reasonable to give more time on the first turn or two while players are still learning the position.
- **Real-time pacing**: avoid dead time between meaningful decisions -- a lull where nothing the player does matters is where disengagement happens. Consider ramping tempo or difficulty over the course of a match rather than holding it flat throughout, and size the match/round length to the genre's realistic attention span (a 90-second round and a 20-minute one need very different pacing curves, not just a longer version of the same one).
- **Eliminated-but-not-done players**: a player knocked out before the match ends but still nominally part of the session has the same downtime problem as a slow turn-based wait. Decide deliberately what they get -- spectating the rest of the match, a lightweight side activity, or a clean early exit -- rather than defaulting to whatever screen happens to already exist for a different case.

## No Pause Menu

**Do not use a pause menu in a game with other real players.** Pausing implies halting the shared game loop, which either freezes it for every other player (bad UX, and exploitable -- a losing player can pause to stall) or only pauses locally while the server and other players keep moving, producing desync when the local player resumes. Neither is acceptable in a match with other real people in it.

What replaces it depends on game type:

- **Real-time games**: a non-blocking settings/options overlay (mute, controls reference, leave/forfeit confirmation) that layers on top of a running game loop without stopping it.
- **Turn-based games**: no pause needed -- use the per-turn timer and idle-timeout policy from the section above.
- **Private test/practice sessions**: since these are informal sessions with people you invited, a mutually-agreed pause is lower-risk -- but still not the default; treat it as an explicit opt-in feature, not baseline architecture.

## Competitive Balance and Fairness

### Turn Order and First-Player Advantage

In many games, turn order itself is an advantage -- going first often means more information, a head start on resource accumulation, or first claim on limited options (even chess has a measurable first-move advantage). Decide turn order deliberately rather than defaulting to "player 1 goes first, always": random order is fair but can feel arbitrary if the same player keeps winning the roll across a session; rotating who goes first each match/round is fair across a series but not within a single match; and compensation (e.g. giving the second player an extra resource) can offset a known first-move edge directly. If first-move advantage is significant in your game's math, address it explicitly with one of these rather than leaving it as an unexamined default.

### Catch-Up Mechanics and the Runaway Leader Problem

A "runaway leader" is a player whose early lead becomes self-reinforcing: having more resources/territory/score makes it easier to gain even more, a positive feedback loop that makes the outcome feel decided long before the match ends (the classic example is *Settlers of Catan*, where more settlements produce more resources to build more settlements). If your scoring, resource, or matchmaking systems have this shape, expect losing players to disengage before the match is actually over.

Catch-up mechanics deliberately counteract this by rewarding players who are behind and/or constraining players who are ahead -- e.g. *Power Grid*'s turn order and resource-buying priority both favor the player currently in last place. This is a real design tradeoff, not a free lunch: making comebacks easier also makes a skilled, clean win feel less rewarding, and heavy-handed catch-up mechanics can feel like the game is punishing good play.

Push it too far in the other direction and you get the opposite failure mode: **rubber-banding** so strong that early-to-mid-match decisions stop mattering at all, because the mechanic will equalize the outcome regardless of how well anyone played until the very end. *Mario Kart*'s blue shell is the canonical example -- a catch-up item powerful and reliable enough that racing skill for most of the track barely affects who wins, which is exactly backwards for a game that's nominally about racing skill. Both failure modes are the same underlying question at different settings: does an early lead make the rest of the match *easier* or *harder* to keep winning? Too easy is a runaway leader; too hard (i.e. a lead barely helps at all) is rubber-banding. Tune catch-up strength to narrow the gap, not erase it -- especially for modes gated behind stats like win-count that already select for players who are ahead.

### Kingmaking

Distinct from the runaway leader problem, and specific to games with 3+ players: **kingmaking** is when a player who can no longer win still has enough remaining influence to decide which of the other players does win -- typically through who they choose to attack, help, or trade with on their way out. It's a worse problem than it sounds, because it decouples the outcome from the leading players' actual play: the better player doesn't necessarily win, whoever the kingmaker likes more (or resents less) does. Kingmaking specifically requires the deciding player to gain nothing themselves from the choice -- if every option still affects their own standing, it's just normal strategy, not kingmaking.

Common mitigations: hidden victory-condition information (a player who doesn't know they've already lost is less likely to deliberately kingmake), reducing direct player-to-player targeting late in the game, or removing a clearly-eliminated player's ability to affect other players' outcomes at all (bot them out or end their turns automatically) rather than leaving them in with nothing to lose.

### Symmetric vs. Asymmetric Balance

Symmetric games (every player has access to the same options) are balanced by making sure no single option or strategy strictly dominates the others -- if one path is always correct, every other option is decorative, and the game is smaller than it looks. This is the more tractable balance problem: playtesting mostly needs to confirm no single strategy wins too often.

Asymmetric games (different players have different roles, factions, or abilities by design -- a hidden-traitor mode, a "one vs. many" mode, different playable factions) can't be balanced by making the roles identical without erasing the point of the asymmetry. Instead, balance for aggregate win-rate parity across many matches at similar skill levels -- does the traitor role win about as often as the crew, over enough games -- not mechanical mirroring. This is the harder problem, and it's empirical, not something you can fully solve at a whiteboard: budget for actual playtesting data (win rates, pick rates) and expect to keep tuning after launch, not just before it.

## Hidden Information and Anti-Cheat Architecture

This is a sharper, more specific version of "server-authoritative state" above, and it matters most for card games and anything with fog-of-war-style partial visibility: **never send a client data it isn't allowed to see, even if the UI hides it.** A client that holds another player's hand, an unrevealed card, or unexplored map state in memory has that data sitting in the browser regardless of whether anything on screen displays it -- it's trivially extractable via devtools or a modified client. Hiding secret state in the UI is not the same as not sending it.

Concretely:

- **Card/hidden-hand games**: the server holds every player's hand; each client receives only its own cards plus counts (not contents) for other players' hands.
- **Fog-of-war / partial-visibility games**: filter world state to each client's actual vision server-side before sending, rather than sending the full world state and relying on the renderer not to draw the hidden parts.
- **Simultaneous-action games** (both players choose at once, e.g. rock-paper-scissors-style or sealed bidding): use a commit-then-reveal pattern -- both players' choices are submitted and locked server-side before either client learns the other's choice, preventing a player from waiting to see the opponent's move before finalizing their own.

## Cooperative Game Pitfalls: Quarterbacking

If a mode is cooperative rather than competitive, the failure mode shifts from "unfair advantage" to "the game stops being cooperative in practice" -- specifically, one player ("the alpha player" or "quarterback") ends up directing every other player's moves, and the rest of the table becomes an audience. This is a well-documented problem in tabletop co-op design (*Pandemic* is the canonical example) and it degrades the experience even though no rule was technically broken.

Design mitigations, in roughly increasing order of restrictiveness: give each player private information the others can't see (so no single player has the full picture needed to direct everyone), keep hands/resources visually separated per player rather than pooled on a shared board, and where necessary, limit how much players can discuss before acting. Pick the mitigation that fits the game's tone -- restricting communication entirely is a strong intervention and can feel antisocial if the game is meant to be chatty.

## Pre-Ship Validation Checklist (Multiplayer)

Run this in addition to the generic checklist in `game-architecture`:

- [ ] **Server authority defined** -- every value that affects other players or the match outcome is validated or computed server-side, not trusted from a client
- [ ] **Disconnect handling complete** -- detection, grace period, full-snapshot resync, and expiry behavior are all explicitly defined and tested (not left to hang)
- [ ] **No pause menu** blocking the shared match; the appropriate replacement (overlay or turn timer) is in place
- [ ] **Idle/waiting states designed** -- turn-based waiting, real-time pacing, and eliminated-but-not-done players all have a deliberate experience, not a blank screen
- [ ] **Hidden information never sent** to a client that shouldn't see it, even for display-only purposes
- [ ] **Simultaneous-action games** use commit-then-reveal, not "first client to submit wins the information advantage"
- [ ] **Latency fairness considered** -- if the game is fast-paced and precision-timing matters, a deliberate lag-compensation approach is chosen, not left to whatever the networking library defaults to
- [ ] **Balance check** -- no early lead makes the rest of the match strictly easier without counterbalance (runaway leader), no catch-up mechanic strong enough to make early play irrelevant (rubber-banding), turn order/first-move advantage addressed, and (3+ players) no player can kingmake without cost to themselves
- [ ] **Asymmetric roles, if any**, are balanced for win-rate parity across playtests, not mechanical mirroring
- [ ] **Cooperative modes** -- if applicable, some mechanism (private info, separated hands) prevents one player from directing everyone else

## Further Reading

If you want to go deeper on any of the above, these are the primary sources this skill draws from:

- Client-side prediction, server reconciliation, entity interpolation: Gabriel Gambetta, "Fast-Paced Multiplayer" (gabrielgambetta.com/client-server-game-architecture.html)
- Rollback netcode: GGPO (ggpo.net) and "Netcode Architectures Part 2: Rollback" (snapnet.dev/blog/netcode-architectures-part-2-rollback/)
- Latency fairness / lag compensation: Valve Developer Community, "Lag Compensation" (developer.valvesoftware.com/wiki/Lag_compensation)
- Reconnection design: "How to Successfully Create a Reconnect Ability in Multiplayer Games" (getgud.io)
- Turn-based sync via reducer/command pattern: "Making a turn-based multiplayer game in Rust" (herluf-ba.github.io)
- Turn order and first-player advantage: "Turn Order Variations: Design Tips for Game Creators" (minifiniti.com)
- Catch-up mechanics and the runaway leader problem: George Skaff Elias, Richard Garfield, K. Robert Gutschera, *Characteristics of Games* (MIT Press); "The Runaway Leader 'Problem'" (insideupgames.com)
- Rubber-banding / overcorrected catch-up: "Theory: Rubber Bands" (lawofgamedesign.com); Blue Shell (Wikipedia) as the canonical example
- Kingmaking: "Mitigating Kingmaking in Multiplayer Board Games" (diva-portal.org)
- Symmetric vs. asymmetric balance: "Symmetrical vs. Asymmetrical Balance in Game Design" (davidgagnon.wordpress.com)
- Hidden information and anti-cheat: "Preventing Cheaters in Fog Of War Games" (edward-thomson.medium.com)
- Cooperative-game quarterbacking: "Mitigating quarterbacking in cooperative games" (donteatthemeeples.com)
- MDA framework: Wikipedia summary (en.wikipedia.org/wiki/MDA_framework)

Note: pre-match lobby, presence, and matchmaking UX are intentionally not covered here -- on platforms that provide matchmaking, that surface usually belongs to the platform rather than the individual game. If you do own that surface yourself, general multiplayer UX writing on lobbies/matchmaking (e.g. Photon's or PlayFab's matchmaking documentation) covers it better than a board-game-theory-flavored skill would.

## Note on this skill

Original content, not adapted from a single upstream source -- synthesized from public multiplayer game networking practice (client/server architecture, rollback netcode, lag compensation) and tabletop/board-game design theory (turn order, catch-up mechanics, kingmaking, hidden information, cooperative-game dynamics), since the design problems in multiplayer games are older and better-studied outside video games than within them. No platform branding; applies to any team building a game with more than one real player, on any platform. Chat/communication system implementation is intentionally out of scope here -- see `game-architecture`.
