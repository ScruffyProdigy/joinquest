---
name: game-architecture
description: >-
  Code-level architecture patterns for browser games -- EventBus, centralized GameState,
  constants, module layering, restart-safety, communication systems (chat/emotes/quick-chat),
  and implementing client-side prediction/server reconciliation ("predict and rewind"). Use when
  structuring a new game project, organizing game code, deciding how systems should communicate,
  or making architectural decisions about game code. Applies to single-player and multiplayer
  games alike. For multiplayer-specific design concerns (client/server authority,
  disconnect/reconnect, turn structure, competitive balance, anti-cheat, lobby UX), use the
  multiplayer-game-design skill instead.
---

# Game Architecture Patterns

Reference knowledge for building well-structured browser games. These patterns apply to both Three.js (3D) and Phaser (2D) games, and to single-player or multiplayer games equally -- this file covers how the code is organized, not what a multiplayer game specifically needs to handle (see `multiplayer-game-design` for that).

## Core Principles

1. **Core Loop First**: Implement the minimum gameplay loop before any polish. The order is: input -> movement -> fail condition -> scoring -> restart. Only after the core loop works should you add visuals, audio, or juice. Keep initial scope small: 1 scene/level, 1 mechanic, 1 fail condition.

2. **Event-Driven Communication**: Modules never import each other for communication. All cross-module messaging goes through a singleton EventBus with predefined event constants.

3. **Centralized State**: A single GameState singleton holds all game state. Systems read state directly and modify it through events. No scattered state across modules.

4. **Configuration Centralization**: Every magic number, balance value, asset path, spawn point, and timing value goes in `Constants.js`. Game logic files contain zero hardcoded values.

5. **Orchestrator Pattern**: One `Game.js` class initializes all systems, manages game flow (boot -> gameplay -> death/win -> restart), and runs the main loop. Systems don't self-initialize. **No title screen by default** -- boot directly into gameplay. Only add a title/menu scene if the user explicitly asks for one.

6. **Restart-Safe and Deterministic**: Gameplay must survive full restart cycles cleanly. `GameState.reset()` restores a complete clean slate. All event listeners are removed in cleanup/shutdown. No stale references, lingering timers, leaked tweens, or orphaned physics bodies survive across restarts. Test by restarting 3x in a row -- the third run must behave identically to the first.

7. **Clear Separation of Concerns**: Code is organized into functional layers:
   - `core/` - Foundation (Game, EventBus, GameState, Constants)
   - `systems/` - Engine-level systems (input, physics, audio, particles)
   - `gameplay/` - Game mechanics (player, enemies, weapons, scoring)
   - `level/` - World building (level construction, asset loading)
   - `ui/` - Interface (menus, HUD, overlays)

## Event System Design

### Event Naming Convention

Use `domain:action` format grouped by feature area:

```js
export const Events = {
  // Player
  PLAYER_DAMAGED: 'player:damaged',
  PLAYER_HEALED: 'player:healed',
  PLAYER_DIED: 'player:died',

  // Enemy
  ENEMY_SPAWNED: 'enemy:spawned',
  ENEMY_KILLED: 'enemy:killed',

  // Game flow
  GAME_STARTED: 'game:started',
  GAME_PAUSED: 'game:paused',
  GAME_OVER: 'game:over',

  // System
  ASSETS_LOADED: 'assets:loaded',
  LOADING_PROGRESS: 'loading:progress'
};
```

### Event Data Contracts

Always pass structured data objects, never primitives:

```js
// Good
eventBus.emit(Events.PLAYER_DAMAGED, { amount: 10, source: 'enemy', damageType: 'melee' });

// Bad
eventBus.emit(Events.PLAYER_DAMAGED, 10);
```

**Multiplayer note:** the EventBus pattern governs how modules communicate *locally* -- it doesn't by itself decide whether an event like `PLAYER_DAMAGED` should fire immediately from local input (prediction) or only once the server confirms it. See Client-Side Prediction and Server Reconciliation below for how that's actually implemented, and `multiplayer-game-design`'s Client/Server Authority section for when it's worth the complexity.

## State Management

### GameState Structure

Organize state into clear domains:

```js
class GameState {
  constructor() {
    this.player = { health, maxHealth, speed, inventory, buffs };
    this.combat = { killCount, waveNumber, score };
    this.game = { started, paused, isPlaying };
  }
}
```

**Multiplayer note:** this single-player-shaped example doesn't generalize directly. With more than one real player, `player` becomes a collection keyed by player ID (`players: { [playerId]: { health, ... } }`), and GameState typically needs to distinguish local/predicted state from server-confirmed state rather than treating "the state" as one flat object. See Client-Side Prediction and Server Reconciliation below for what that split looks like in code, and `multiplayer-game-design`'s Client/Server Authority section for when it's worth the complexity.

## Client-Side Prediction and Server Reconciliation (the "Predict and Rewind" Pattern)

For real-time multiplayer games, waiting for a server round-trip before showing the result of a player's own input feels laggy even on a good connection. The standard fix: apply input locally right away (predict), and correct later if the server disagrees (reconcile). This is the concrete implementation behind the client-side prediction mentioned in the notes above and in `multiplayer-game-design`'s Client/Server Authority section.

1. **Tag and buffer every input.** Give each input command an incrementing sequence number, and keep a buffer of commands the server hasn't confirmed yet:

```js
let inputSeq = 0;
const pendingInputs = []; // trim from the front as the server confirms commands

function sendInput(input) {
  const cmd = { seq: inputSeq++, input, dt: getDeltaTime() };
  pendingInputs.push(cmd);
  applySimulationStep(localState, cmd); // predict immediately, don't wait for the server
  network.send('input', cmd);
}
```

2. **Apply input through one deterministic function**, used both for live prediction and for replay during reconciliation. `applySimulationStep(state, cmd)` must be a pure function -- same `(state, cmd)` in, same resulting `state` out, every time, with no randomness and no side effects like spawning particles or playing sounds inside it. Cosmetic effects belong in events fired only when a command is confirmed, not predicted, or they double-fire on replay.

3. **The server processes inputs in order** and periodically broadcasts an authoritative snapshot that includes the sequence number of the last input it actually processed for that client (`ackSeq`).

4. **Reconcile whenever a snapshot arrives** -- discard confirmed inputs, snap to the authoritative state, then replay whatever's left:

```js
function onServerSnapshot(snapshot) {
  // Drop anything the server already processed
  while (pendingInputs.length && pendingInputs[0].seq <= snapshot.ackSeq) {
    pendingInputs.shift();
  }

  // Server truth replaces local guesswork
  localState = cloneState(snapshot.state);

  // Re-simulate only the inputs the server hasn't seen yet
  for (const cmd of pendingInputs) {
    applySimulationStep(localState, cmd);
  }
}
```

Because replay uses the exact same `applySimulationStep` as live prediction, a correct prediction produces a no-op reconciliation -- the player only sees a correction when their guess was actually wrong, not on every snapshot.

5. **Remote players get interpolation, not prediction.** You don't know another player's future input, so predicting it produces worse visible corrections than just accepting a bit of latency. Render remote entities by interpolating between the last two received snapshots (effectively drawing them slightly in the past) instead of running `applySimulationStep` on their behalf.

6. **Rollback netcode takes the same pattern further**, for genres where sub-frame timing decides outcomes (fighting games, precise platformer duels): predict every frame, including a guess at remote players' input (usually "repeat their last input"), and when real remote input arrives late and differs from the guess, roll the simulation back to the last confirmed frame and resimulate forward to the present. This requires the whole simulation to resimulate several frames fast enough to be invisible, plus total determinism. See `multiplayer-game-design`'s Turn Structure section for when this is worth adopting (GGPO is the reference implementation) versus when plain prediction/reconciliation above is enough.

### What this pattern requires from the rest of your architecture

- `applySimulationStep` (or whatever your core update function is) must be pure and deterministic on both client and server -- this is a stricter requirement than typical single-player code, where a stray `Math.random()` or DOM read inside your update loop is harmless.
- Cosmetic/one-shot effects (particles, sounds, screen shake) can't live inside that function -- fire them from confirmed events only, or gate them by sequence number so replay doesn't re-trigger them.
- `GameState` needs a cheap clone/restore path, not just `reset()` -- reconciliation clones state on every snapshot, so this needs to be fast as well as correct.

## Game Flow

Standard flow for a single-player scene:

```
Boot/Load -> Gameplay <-> Pause Menu (if requested)
                      -> Game Over -> Gameplay (restart)
```

**No title screen by default.** Games boot directly into gameplay. Only add a title/menu scene if the user explicitly requests one.

**Building a multiplayer game?** This diagram doesn't hold as-is: boot has to include connecting and claiming a seat, gameplay doesn't start until enough players are present, and "Game Over" is typically a synchronized match end rather than a per-client restart button. A pause menu is also the wrong pattern once other real players share the game loop. See `multiplayer-game-design`'s Turn Structure and Session Continuity sections for the actual multiplayer flow shape, not just the pause-menu replacement.

## Communication Systems (Chat, Emotes, Quick-Chat)

If the game includes any player-to-player communication, treat it as its own system (e.g. `systems/CommunicationSystem.js`) wired through the EventBus like any other system, not bolted onto the UI layer directly.

- **Quick-chat/canned phrases** ("Nice shot!", "Good game") are the safer default -- define the phrase set in `Constants.js` like every other config value, not hardcoded in UI components. Easy to localize, and structurally can't be used to harass another player the way free text can.
- **Free-text chat** needs a moderation/profanity-filter pass as a distinct server-side step before a message reaches other clients -- a client-side-only filter can be bypassed by a modified client. Rate-limit message frequency per player to prevent flooding.
- **Emotes** are typically the lowest-risk option -- a fixed, art-asset-backed set of reactions. Still belongs in `Constants.js`, not inline in component code.
- Whether to allow free text at all (versus quick-chat only) is a design decision, not just an implementation one -- restricting it usually helps competitive games and can hurt cooperative/social ones. See `multiplayer-game-design` for that tradeoff; this section only covers how to structure whichever option you pick.

## Common Architecture Pitfalls

- **Unwired physics bodies** -- Creating a static physics body (e.g., ground, wall) without wiring it to other bodies via `physics.add.collider()` or `physics.add.overlap()` has no gameplay effect. Every boundary or obstacle needs explicit collision wiring to the entities it should interact with. After creating any static body, immediately add the collider call.
- **Interactive elements blocked by overlapping display objects** -- When building UI (buttons, menus), the topmost display object in the scene list receives pointer events. Never hide the interactive element behind a decorative layer. Either make the visual element itself interactive, or ensure nothing is rendered on top of the hit area.
- **Polish before gameplay** -- Adding particles, screen shake, and transitions before the core loop works is a common time sink. Get input -> action -> fail condition -> scoring -> restart working first. Everything else is polish.
- **No cleanup on restart** -- Forgetting to remove event listeners, destroy timers, and dispose resources in `shutdown()` causes ghost behavior, double-firing events, and memory leaks after restart.

## Pre-Ship Validation Checklist

- [ ] **Core loop** -- Player can start, play, lose/win, and see the result
- [ ] **Restart** -- Works cleanly 3x in a row with identical behavior
- [ ] **Mobile input** -- Touch/tap/swipe/gyro works; 44px minimum tap targets
- [ ] **Desktop input** -- Keyboard + mouse works
- [ ] **Responsive** -- Canvas resizes correctly on window resize
- [ ] **Constants** -- Zero hardcoded magic numbers in game logic
- [ ] **EventBus** -- No direct cross-module imports for communication
- [ ] **Cleanup** -- All listeners removed in shutdown, resources disposed
- [ ] **Delta-based** -- All movement uses delta time, not frame count
- [ ] **Build** -- `npm run build` succeeds with no errors
- [ ] **No errors** -- No uncaught exceptions or console errors at runtime
- [ ] **Communication system** (if any) -- routed through EventBus, free text has server-side moderation, phrase/emote sets live in Constants
- [ ] **Prediction/reconciliation** (if real-time multiplayer) -- simulation step is pure and deterministic, cosmetic effects fire only on confirmation, remote entities use interpolation rather than prediction
- [ ] **Multiplayer?** -- If this game has more than one real player, also run the checklist in `multiplayer-game-design` (server authority, disconnect handling, no pause menu)

## Note on this adapted copy

Adapted from the open-source `game-creator` plugin (PlayableIntelligence/game-creator, MIT licensed) for this repo. The companion `system-patterns.md` (object pooling, delta-time normalization, resource disposal, wave/spawn systems, buff/powerup system, haptic feedback, asset management) was not bundled -- pull it from the source repo if those implementations are needed. Play.fun-specific safe-zone/monetization guidance from the original has been removed since it doesn't apply here. Multiplayer-specific content (client/server split, session continuity, competitive balance, anti-cheat, lobby UX) was split out into the separate `multiplayer-game-design` skill so this file stays focused on single-scene code structure and triggers correctly for solo-mode work.
