# Game Design Skills for JoinQuest Developers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship generic multiplayer game-architecture skills (`game-architecture`, `phaser`, `threejs-game`) plus a thin JoinQuest-specific mapping doc (`game-client-patterns.md`), wired into Phase 9 of the existing `joinquest-integration` skill, so developers (and their coding agents) get real architectural guidance for building the game itself — not just the integration contract.

**Architecture:** Three skill folders land in `.agents/skills/` alongside the existing `joinquest-integration` skill. Two stay fully generic (no JoinQuest branding); the third (`game-architecture`) is also generic but lives in this repo because it's the thing `joinquest-integration` needs to point developers to. The only JoinQuest-specific content is `game-client-patterns.md`, added inside the existing `joinquest-integration` skill folder. `docs/developer-agent-playbook.md` (canonical) gets its Phase 9 rewritten to reference the new skills, then `scripts/sync-developer-docs.sh` propagates that to the three existing embedded copies. The sync script also gets extended to copy the three new skill folders into `plugins/joinquest/skills/` so they ship as part of the installable JoinQuest plugin, same distribution path as `joinquest-integration` today.

**Tech Stack:** Markdown skill files (Claude Agent Skills format, YAML frontmatter + Markdown body), bash (sync script), Python 3 (frontmatter validation only — no application code changes in this plan).

**Note on "tests" in this plan:** this is a documentation/skill-content project, not application code. "Tests" here mean: (a) YAML frontmatter parses and has required fields, (b) referenced files/links actually exist, (c) content-presence checks (grep) for required sections. Where a task touches real application files (the sync script), the test is running it and diffing the output against expected content.

## Global Constraints

- Skill `name`/`description` frontmatter only — no `argument-hint`, `license`, or `metadata` fields, matching the existing `joinquest-integration` skill's convention (simpler than the upstream `game-creator` frontmatter these are adapted from).
- Anything platform-specific gets built into `game-client-patterns.md` or the `joinquest-integration` skill itself. `game-architecture`, `phaser`, `threejs-game` stay fully generic — no JoinQuest branding, no references to JoinQuest's API contract.
- `docs/developer-agent-playbook.md` is the **canonical** source for playbook content. Never hand-edit `.agents/skills/joinquest-integration/playbook.md`, `backend/internal/developer/agent_playbook.md`, or `plugins/joinquest/skills/joinquest-integration/playbook.md` directly — always edit the canonical doc, then run `./scripts/sync-developer-docs.sh`.
- MIT attribution to the source project (`PlayableIntelligence/game-creator`) is preserved in each adapted skill's closing "Note on this adapted copy" section.

---

### Task 1: Add the `game-architecture` skill to the repo

**Files:**
- Create: `.agents/skills/game-architecture/SKILL.md`

**Interfaces:**
- Produces: a skill named `game-architecture`, triggered on "designing game systems, planning architecture, structuring a game project, or making architectural decisions about game code" (per its `description` frontmatter). Task 6 references this skill by name from Phase 9.

- [ ] **Step 1: Create the file with the full adapted skill content**

```markdown
---
name: game-architecture
description: Game architecture patterns and best practices for browser games. Use when designing game systems, planning architecture, structuring a game project, or making architectural decisions about game code.
---

# Game Architecture Patterns

Reference knowledge for building well-structured browser games. These patterns apply to both Three.js (3D) and Phaser (2D) games.

## Core Principles

1. **Core Loop First**: Implement the minimum gameplay loop before any polish. The order is: input -> movement -> fail condition -> scoring -> restart. Only after the core loop works should you add visuals, audio, or juice. Keep initial scope small: 1 scene/level, 1 mechanic, 1 fail condition.

2. **Event-Driven Communication**: Modules never import each other for communication. All cross-module messaging goes through a singleton EventBus with predefined event constants.

3. **Centralized State**: A single GameState singleton holds all game state. Systems read state directly and modify it through events. No scattered state across modules.

4. **Configuration Centralization**: Every magic number, balance value, asset path, spawn point, and timing value goes in `Constants.js`. Game logic files contain zero hardcoded values.

5. **Orchestrator Pattern**: One `Game.js` class initializes all systems, manages game flow (boot -> gameplay -> death/win -> restart), and runs the main loop. Systems don't self-initialize. **No title screen by default** — boot directly into gameplay. Only add a title/menu scene if the user explicitly asks for one.

6. **Restart-Safe and Deterministic**: Gameplay must survive full restart cycles cleanly. `GameState.reset()` restores a complete clean slate. All event listeners are removed in cleanup/shutdown. No stale references, lingering timers, leaked tweens, or orphaned physics bodies survive across restarts. Test by restarting 3x in a row — the third run must behave identically to the first.

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

## Client / Server Split

Every multiplayer game splits responsibility between a client (rendering, input, local feel) and a server (the shared source of truth). Getting this split wrong is the single most common architecture mistake in multiplayer games — most bugs, cheating vectors, and "worked in testing, broke with real players" incidents trace back to state living in the wrong place.

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

- **Trusting client-reported outcomes** — client says "I won," server just believes it. Fine for a solo game, a real bug in anything competitive or cooperative.
- **State that only exists in one client's memory** — if the server never learns about it, it can't be part of a reconnect resync (see Multiplayer Session Continuity below) and other players have no way to know about it.
- **Recomputing authoritative values from client input without validation** — e.g. accepting a client-sent "damage dealt" number instead of the server independently computing it from the actual game rules.

This split is also what makes the disconnect/reconnect pattern below possible at all — you can only resync a reconnecting player from a source of truth that isn't the thing that just disconnected.

## Game Flow

**Single-player:**

```
Boot/Load -> Gameplay <-> Pause Menu (if requested)
                      -> Game Over -> Gameplay (restart)
```

**Multiplayer:**

```
Boot/Load -> Gameplay <-> Non-blocking settings overlay
                      -> Disconnect (grace period, see below)
                      -> Match Over -> Gameplay (restart, if rematch supported)
```

**Do not use a pause menu in multiplayer games.** Pausing implies halting the shared game loop, which either freezes it for every other player (bad UX, and exploitable — a losing player can pause to stall) or only pauses locally while the server and other players keep moving, producing desync when the local player resumes. Neither is acceptable in a match with other real players.

What replaces it depends on game type:

- **Real-time games**: a non-blocking settings/options overlay (mute, controls reference, leave/forfeit confirmation) that layers on top of a running game loop without stopping it.
- **Turn-based games**: no pause needed — use a per-turn timer with an idle-timeout policy (auto-skip the turn, auto-forfeit after N missed turns) instead.
- **Private test/practice sessions**: since these are informal sessions with people you invited, a mutually-agreed pause is lower-risk — but still not the default; treat it as an explicit opt-in feature, not baseline architecture.

**No title screen by default.** Games boot directly into gameplay. Only add a title/menu scene if the user explicitly requests one.

## Multiplayer Session Continuity (Disconnect / Reconnect)

For any match-based multiplayer game, this is the design question that actually matters in place of pausing: **what happens when a player's connection drops or their tab loses focus mid-match?** Players close laptops, switch tabs, and lose wifi far more often than they intentionally quit — the game needs a policy for this, not just a happy path.

### Principles

1. **Server-authoritative state is required for reconnect to work at all.** If match state lives only in each client's memory, a disconnected client has nothing to resync from when it comes back. Keep a server-side source of truth for anything that must survive a reconnect.
2. **Detect disconnect explicitly** — WebSocket `close`, `beforeunload`, and `visibilitychange` (tab backgrounded) are different signals with different intent; a backgrounded tab on mobile is not the same as an intentional quit, so don't treat them identically.
3. **Grace period before removing a player from the match** — give a disconnected player a window (e.g. 15–30s, tune per game pace) during which their seat is reserved and the match continues without them, rather than instantly forfeiting or ending the match. Communicate this state to remaining players ("Waiting for Player 2 to reconnect...") rather than leaving it silent.
4. **Reconnect resyncs from server truth, not deltas** — when a player reclaims their seat, send a full authoritative state snapshot, not an incremental patch. The client should never have to guess what it missed.
5. **Define the grace-period expiry behavior per game** — options include: forfeit the disconnected player, hand them to an AI/bot for the remainder, or end the match for everyone with a clear reason shown to remaining players. Pick one deliberately; don't leave it undefined and let the client silently hang.
6. **Reconnect UI is part of the core loop, not polish** — a `Reconnecting...` state with a visible timer belongs in the same tier as the win/lose screen, not bolted on afterward.

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

## Common Architecture Pitfalls

- **Unwired physics bodies** — Creating a static physics body (e.g., ground, wall) without wiring it to other bodies via `physics.add.collider()` or `physics.add.overlap()` has no gameplay effect. Every boundary or obstacle needs explicit collision wiring to the entities it should interact with. After creating any static body, immediately add the collider call.
- **Interactive elements blocked by overlapping display objects** — When building UI (buttons, menus), the topmost display object in the scene list receives pointer events. Never hide the interactive element behind a decorative layer. Either make the visual element itself interactive, or ensure nothing is rendered on top of the hit area.
- **Polish before gameplay** — Adding particles, screen shake, and transitions before the core loop works is a common time sink. Get input -> action -> fail condition -> scoring -> restart working first. Everything else is polish.
- **No cleanup on restart** — Forgetting to remove event listeners, destroy timers, and dispose resources in `shutdown()` causes ghost behavior, double-firing events, and memory leaks after restart.

## Pre-Ship Validation Checklist

- [ ] **Core loop** — Player can start, play, lose/win, and see the result
- [ ] **Restart** — Works cleanly 3x in a row with identical behavior
- [ ] **Mobile input** — Touch/tap/swipe/gyro works; 44px minimum tap targets
- [ ] **Desktop input** — Keyboard + mouse works
- [ ] **Responsive** — Canvas resizes correctly on window resize
- [ ] **Constants** — Zero hardcoded magic numbers in game logic
- [ ] **EventBus** — No direct cross-module imports for communication
- [ ] **Cleanup** — All listeners removed in shutdown, resources disposed
- [ ] **Delta-based** — All movement uses delta time, not frame count
- [ ] **Build** — `npm run build` succeeds with no errors
- [ ] **No errors** — No uncaught exceptions or console errors at runtime
- [ ] **Disconnect handling** (multiplayer only) — Grace period, reconnect resync, and expiry behavior are all defined and tested; no pause menu blocking the shared match

## Note on this adapted copy

Adapted from the open-source `game-creator` plugin (PlayableIntelligence/game-creator, MIT licensed) for this repo. The companion `system-patterns.md` (object pooling, delta-time normalization, resource disposal, wave/spawn systems, buff/powerup system, haptic feedback, asset management) was not bundled — pull it from the source repo if those implementations are needed. Play.fun-specific safe-zone/monetization guidance from the original has been removed since it doesn't apply here. Client/server split and multiplayer session continuity content is original to this repo, not present in the upstream skill.
```

- [ ] **Step 2: Validate frontmatter parses**

Run:
```bash
cd /path/to/lobby
python3 -c "
import re, yaml
content = open('.agents/skills/game-architecture/SKILL.md').read()
fm = re.match(r'^---\n(.*?)\n---\n', content, re.DOTALL).group(1)
data = yaml.safe_load(fm)
assert data['name'] == 'game-architecture'
assert 'description' in data and len(data['description']) > 20
print('OK:', data['name'])
"
```
Expected: `OK: game-architecture`

- [ ] **Step 3: Verify required sections are present**

Run:
```bash
grep -c "^## " .agents/skills/game-architecture/SKILL.md
grep -q "Client / Server Split" .agents/skills/game-architecture/SKILL.md && echo "split: present"
grep -q "Multiplayer Session Continuity" .agents/skills/game-architecture/SKILL.md && echo "continuity: present"
grep -q "Do not use a pause menu" .agents/skills/game-architecture/SKILL.md && echo "no-pause rule: present"
```
Expected: a section count of 9, and all three "present" lines printed.

- [ ] **Step 4: Commit**

```bash
git add .agents/skills/game-architecture/SKILL.md
git commit -m "Add generic game-architecture skill (client/server split, session continuity)"
```

---

### Task 2: Add the `phaser` skill to the repo

**Files:**
- Create: `.agents/skills/phaser/SKILL.md`

**Interfaces:**
- Produces: a skill named `phaser`, triggered on "creating a new 2D game, adding 2D game features, working with Phaser, or building sprite-based web games." Referenced by name from Phase 9 (Task 6).

- [ ] **Step 1: Create the file with the full adapted skill content**

```markdown
---
name: phaser
description: >
  Build 2D browser games with Phaser 3 using scene-based architecture and centralized state.
  Use when creating a new 2D game, adding 2D game features, working with Phaser, or building
  sprite-based web games.
---

# Phaser 3 Game Development

You are an expert Phaser game developer. Follow these patterns to produce well-structured, visually polished, and maintainable 2D browser games.

## Core Principles

1. **Core loop first** — Implement the minimum gameplay loop before any polish: boot → preload → create → update. Add the win/lose condition and scoring **before** visuals, audio, or juice. Keep initial scope small: 1 scene, 1 mechanic, 1 fail condition.
2. **TypeScript-first** — Always use TypeScript for type safety and IDE support
3. **Scene-based architecture** — Each game screen is a Scene; keep them focused
4. **Vite bundling** — Use the official `phaserjs/template-vite-ts` template
5. **Composition over inheritance** — Prefer composing behaviors over deep class hierarchies
6. **Data-driven design** — Define levels, enemies, and configs in JSON/data files
7. **Event-driven communication** — All cross-scene/system communication via EventBus
8. **Restart-safe** — Gameplay must be fully restart-safe and deterministic. `GameState.reset()` must restore a clean slate. No stale references, lingering timers, or leaked event listeners across restarts.

## Mandatory Conventions

All games MUST follow these conventions:

- **`core/` directory** with EventBus, GameState, and Constants
- **EventBus singleton** — `domain:action` event naming, no direct scene references
- **GameState singleton** — Centralized state with `reset()` for clean restarts
- **Constants file** — Every magic number, color, speed, and config value — zero hardcoded values
- **Scene cleanup** — Remove EventBus listeners in `shutdown()`

## Project Setup

Use the official Vite + TypeScript template as your starting point:

```bash
npx degit phaserjs/template-vite-ts my-game
cd my-game && npm install
```

### Required Directory Structure

```
src/
├── core/
│   ├── EventBus.ts        # Singleton event bus + event constants
│   ├── GameState.ts       # Centralized state with reset()
│   └── Constants.ts       # ALL config values
├── scenes/
│   ├── Boot.ts            # Minimal setup, start Game scene
│   ├── Preloader.ts       # Load all assets, show progress bar
│   ├── Game.ts            # Main gameplay (starts immediately, no title screen)
│   └── GameOver.ts        # End screen with restart
├── objects/               # Game entities (Player, Enemy, etc.)
├── systems/               # Managers and subsystems
├── ui/                    # UI components (buttons, bars, dialogs)
├── audio/                 # Audio manager, music, SFX
├── config.ts              # Phaser.Types.Core.GameConfig
└── main.ts                # Entry point
```

## Scene Architecture

- **Lifecycle**: `init()` → `preload()` → `create()` → `update(time, delta)`
- Use `init()` for receiving data from scene transitions
- Load assets in a dedicated `Preloader` scene, not in every scene
- Keep `update()` lean — delegate to subsystems and game objects
- **No title screen by default** — boot directly into gameplay. Only add a title/menu scene if the user explicitly asks for one
- Communicate between scenes via EventBus (not direct references)

## Game Objects

- Extend `Phaser.GameObjects.Sprite` (or other base classes) for custom objects
- Use `Phaser.GameObjects.Group` for object pooling (bullets, coins, enemies)
- Use `Phaser.GameObjects.Container` for composite objects, but avoid deep nesting
- Register custom objects with `GameObjectFactory` for scene-level access

## Physics

- **Arcade Physics** — Use for simple games (platformers, top-down). Fast and lightweight.
- **Matter.js** — Use when you need realistic collisions, constraints, or complex shapes.
- Never mix physics engines in the same game.
- Use the **state pattern** for character movement (idle, walk, jump, attack).

## Performance (Critical Rules)

- **Use texture atlases** — Pack sprites into atlases, never load individual images at scale
- **Object pooling** — Use Groups with `maxSize`; recycle with `setActive(false)` / `setVisible(false)`
- **Minimize update work** — Only iterate active objects; use `getChildren().filter(c => c.active)`
- **Camera culling** — Enable for large worlds; off-screen objects skip rendering
- **Batch rendering** — Fewer unique textures per frame = better draw call batching
- **Mobile** — Reduce particle counts, simplify physics, consider 30fps target
- **`pixelArt: true`** — Enable in game config for pixel art games (nearest-neighbor scaling)

## Advanced Patterns

- **ECS with bitECS** — Entity Component System for data-oriented design (used internally by Phaser 4)
- **State machines** — Manage entity behavior states cleanly
- **Singleton managers** — Cross-scene services (audio, save data, analytics)
- **Event bus** — Decouple systems with a shared EventEmitter
- **Tiled integration** — Use Tiled map editor for level design

## Mobile Input Strategy (60/40 Rule)

All games MUST work on desktop AND mobile unless explicitly specified otherwise. Focus 60% mobile / 40% desktop for tradeoffs. Pick the best mobile input for each game concept:

| Game Type | Primary Mobile Input | Desktop Input |
|-----------|---------------------|----------------|
| Platformer | Tap left/right half + tap-to-jump | Arrow keys / WASD |
| Runner/endless | Tap / swipe up to jump | Space / Up arrow |
| Puzzle/match | Tap targets (44px min) | Click |
| Shooter | Virtual joystick + tap-to-fire | Mouse + WASD |
| Top-down | Virtual joystick | Arrow keys / WASD |

Abstract input into an `inputState` object so game logic is source-agnostic, and merge keyboard + touch handling rather than branching on OS detection. Use capability detection (`'ontouchstart' in window || navigator.maxTouchPoints > 0`), not OS-based detection.

### Minimum Entity Sizes for Mobile

Collectibles, hazards, and interactive items must be at least **7–8% of `GAME.WIDTH`** to be recognizable on phone screens. For the main player character, use 12–15% of `GAME.WIDTH`.

## Anti-Patterns (Avoid These)

- **Bloated `update()` methods** — Don't put all game logic in one giant update with nested conditionals. Delegate to objects and systems.
- **Overwriting Scene injection map properties** — Never name your properties `world`, `input`, `cameras`, `add`, `make`, `scene`, `sys`, `game`, `cache`, `registry`, `sound`, `textures`, `events`, `physics`, `matter`, `time`, `tweens`, `lights`, `data`, `load`, `anims`, `renderer`, or `plugins`. These are reserved by Phaser.
- **Creating objects in `update()` without pooling** — This causes GC spikes. Always pool frequently created/destroyed objects.
- **Loading individual sprites instead of atlases** — Each separate texture is a draw call. Pack them.
- **Tightly coupling scenes** — Don't store direct references between scenes. Use EventBus.
- **Ignoring `delta` in update** — Always use `delta` for time-based movement, not frame-based.
- **Deep container nesting** — Containers disable render batching for children. Keep hierarchy flat.
- **Not cleaning up** — Remove event listeners and timers in `shutdown()` to prevent memory leaks and ghost behavior after restart.
- **Hardcoded values** — Every number belongs in `Constants.ts`. No magic numbers in game logic.
- **Unwired physics colliders** — Creating a static body with `physics.add.existing(obj, true)` does nothing on its own. You MUST call `physics.add.collider(bodyA, bodyB, callback)` to connect two bodies.
- **Invisible or hidden button elements** — Never set `setAlpha(0)` on an interactive game object and layer Graphics or other display objects on top. For buttons, always use the Container + Graphics + Text pattern (Container first, Graphics added to container, Text added to container, in that order; Container is the interactive element).

## Pre-Ship Validation Checklist

Before considering a game complete, verify:

- [ ] **Core loop works** — Player can start, play, lose/win, and see the result
- [ ] **Restart works cleanly** — `GameState.reset()` restores a clean slate, no stale listeners or timers
- [ ] **Touch + keyboard input** — Game works on mobile (tap/swipe) and desktop (keyboard/mouse)
- [ ] **Responsive canvas** — `Scale.FIT` + `CENTER_BOTH` + `zoom: 1/DPR` with DPR-multiplied dimensions, crisp on Retina
- [ ] **All values in Constants** — Zero hardcoded magic numbers in game logic
- [ ] **EventBus only** — No direct cross-scene/module imports for communication
- [ ] **Scene cleanup** — All EventBus listeners removed in `shutdown()`
- [ ] **Physics wired** — Every static body has an explicit `collider()` or `overlap()` call
- [ ] **Object pooling** — Frequently created/destroyed objects use Groups with `maxSize`
- [ ] **Delta-based movement** — All motion uses `delta`, not frame count
- [ ] **Build passes** — `npm run build` succeeds with no errors
- [ ] **No console errors** — Game runs without uncaught exceptions or WebGL failures

## Note on this adapted copy

Adapted from the open-source `game-creator` plugin (PlayableIntelligence/game-creator, MIT licensed) for this repo. Companion reference files (`conventions.md`, `project-setup.md`, `scenes-and-lifecycle.md`, `game-objects.md`, `physics-and-movement.md`, `assets-and-performance.md`, `patterns.md`, `no-asset-design.md`, and worked examples) that the original skill links to were not bundled — this file's content stands alone. References to the Play.fun SDK/safe-zone in the original have been removed since they don't apply outside that platform.
```

- [ ] **Step 2: Validate frontmatter parses**

Run:
```bash
python3 -c "
import re, yaml
content = open('.agents/skills/phaser/SKILL.md').read()
fm = re.match(r'^---\n(.*?)\n---\n', content, re.DOTALL).group(1)
data = yaml.safe_load(fm)
assert data['name'] == 'phaser'
assert 'description' in data
print('OK:', data['name'])
"
```
Expected: `OK: phaser`

- [ ] **Step 3: Commit**

```bash
git add .agents/skills/phaser/SKILL.md
git commit -m "Add generic phaser skill"
```

---

### Task 3: Add the `threejs-game` skill to the repo

**Files:**
- Create: `.agents/skills/threejs-game/SKILL.md`

**Interfaces:**
- Produces: a skill named `threejs-game`, triggered on "creating a new 3D game, adding 3D game features, setting up Three.js scenes, or working on any Three.js game project." Referenced by name from Phase 9 (Task 6).

- [ ] **Step 1: Create the file with the full adapted skill content**

```markdown
---
name: threejs-game
description: Build 3D browser games with Three.js using event-driven modular architecture. Use when creating a new 3D game, adding 3D game features, setting up Three.js scenes, or working on any Three.js game project.
---

# Three.js Game Development

You are an expert Three.js game developer. Follow these opinionated patterns when building 3D browser games.

## Performance Notes

- Take your time with each step. Quality is more important than speed.
- Do not skip validation steps — they catch issues early.
- Read the full context of each file before making changes.
- Profile before optimizing. The bottleneck is rarely where you think.

## Tech Stack

- **Renderer**: Three.js (`three@0.183.0+`, ESM imports)
- **Build Tool**: Vite
- **Language**: JavaScript (not TypeScript) for game templates — TypeScript optional
- **Package Manager**: npm

## Project Setup

When scaffolding a new Three.js game:

```bash
mkdir <game-name> && cd <game-name>
npm init -y
npm install three@^0.183.0
npm install -D vite
```

Create `vite.config.js`:

```js
import { defineConfig } from 'vite';

export default defineConfig({
  root: '.',
  publicDir: 'public',
  server: { port: 3000, open: true },
  build: { outDir: 'dist' },
});
```

Add to `package.json` scripts:

```json
{
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  }
}
```

## Modern Import Patterns

### Vite / npm (default)

```js
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
```

### Import Maps / CDN (standalone HTML games, no build step)

```html
<script type="importmap">
{
  "imports": {
    "three": "https://cdn.jsdelivr.net/npm/three@0.183.0/build/three.module.js",
    "three/addons/": "https://cdn.jsdelivr.net/npm/three@0.183.0/examples/jsm/"
  }
}
</script>
```

Use import maps when shipping a single HTML file with no build tooling. Pin the version in the import map URL.

## Required Architecture

Every Three.js game MUST use this directory structure:

```
src/
├── core/
│   ├── Game.js          # Main orchestrator - init systems, render loop
│   ├── EventBus.js      # Singleton pub/sub for all module communication
│   ├── GameState.js     # Centralized state singleton
│   └── Constants.js     # ALL config values, balance numbers, asset paths
├── systems/             # Low-level engine systems
│   ├── InputSystem.js   # Keyboard/mouse/gamepad input
│   ├── PhysicsSystem.js # Collision detection
│   └── ...              # Audio, particles, etc.
├── gameplay/             # Game mechanics
│   └── ...              # Player, enemies, weapons, etc.
├── level/                # Level/world building
│   ├── LevelBuilder.js  # Constructs the game world
│   └── AssetLoader.js   # Loads models, textures, audio
├── ui/                   # User interface
│   └── ...              # Game over, overlays
└── main.js               # Entry point - creates Game instance
```

## Core Principles

1. **Core loop first** — Implement one camera, one scene, one gameplay loop. Add player input and a terminal condition (win/lose) **before** adding visual polish. Keep initial scope small: 1 mechanic, 1 fail condition, 1 scoring system.
2. **Gameplay clarity > visual complexity** — Treat 3D as a style choice, not a complexity mandate. A readable game with simple materials beats a visually complex but confusing one.
3. **Restart-safe** — Gameplay must be fully restart-safe. `GameState.reset()` must restore a clean slate. Dispose geometries/materials/textures on cleanup. No stale references or leaked listeners across restarts.

## Core Patterns (Non-Negotiable)

### 1. EventBus Singleton
ALL inter-module communication goes through an EventBus (`core/EventBus.js`). Modules never import each other directly for communication. Provides `on`, `once`, `off`, `emit`, and `clear` methods. Events use `domain:action` naming (e.g., `player:hit`, `game:over`).

### 2. Centralized GameState
One singleton (`core/GameState.js`) holds ALL game state. Systems read from it, events update it. Must include a `reset()` method that restores a clean slate for restarts.

### 3. Constants File
Every magic number, balance value, asset path, and configuration goes in `core/Constants.js`. Never hardcode values in game logic. Organize by domain: `PLAYER_CONFIG`, `ENEMY_CONFIG`, `WORLD`, `CAMERA`, `COLORS`, `ASSET_PATHS`.

### 4. Game.js Orchestrator
The Game class (`core/Game.js`) initializes everything and runs the render loop. Uses `renderer.setAnimationLoop()` — the official Three.js pattern (handles WebGPU async correctly and pauses when the tab is hidden). Sets up renderer, scene, camera, systems, UI, and event listeners in `init()`.

## Renderer Selection

### WebGLRenderer (default — use for all game templates)

Maximum browser compatibility.

```js
import * as THREE from 'three';
const renderer = new THREE.WebGLRenderer({ antialias: true });
```

### WebGPURenderer (when you need TSL or compute shaders)

Required for custom node-based materials (TSL), compute shaders, and advanced rendering. Import path changes to `'three/webgpu'` and init is async.

```js
import * as THREE from 'three/webgpu';
const renderer = new THREE.WebGPURenderer({ antialias: true });
await renderer.init();
```

**When to pick WebGPU**: You need TSL custom shaders, compute shaders, or node-based materials. Otherwise, stick with WebGL.

## Performance Rules

- **Use `renderer.setAnimationLoop()`** instead of manual `requestAnimationFrame`. It pauses when the tab is hidden and handles WebGPU async correctly.
- **Cap delta time**: `Math.min(clock.getDelta(), 0.1)` to prevent death spirals
- **Cap pixel ratio**: `renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))` — avoids GPU overload on high-DPI screens
- **Object pooling**: Reuse `Vector3`, `Box3`, temp objects in hot loops to minimize GC. Avoid per-frame allocations — preallocate and reuse.
- **Disable shadows on first pass** — Only enable shadow maps when specifically needed and tested on mobile. Dynamic shadows are the single most expensive rendering feature.
- **Keep draw calls low** — Fewer unique materials and geometries = fewer draw calls. Merge static geometry where possible. Use instanced meshes for repeated objects.
- **Prefer simple materials** — Use `MeshBasicMaterial` or `MeshStandardMaterial`. Avoid `MeshPhysicalMaterial`, custom shaders, or complex material setups unless specifically needed.
- **No postprocessing by default** — Skip bloom, SSAO, motion blur, and other postprocessing passes on first implementation. These tank mobile performance. Add only after gameplay is solid and perf budget allows.
- **Keep geometry/material count small** — A game with 10 unique materials renders faster than one with 100. Reuse materials across objects with the same appearance.
- **Use `powerPreference: 'high-performance'`** on the renderer
- **Dispose properly**: Call `.dispose()` on geometries, materials, textures when removing objects
- **Frustum culling**: Let Three.js handle it (enabled by default) but set bounding spheres on custom geometry

## Asset Loading

- Place static assets in `/public/` for Vite
- Use GLB format for 3D models (smaller, single file)
- Use `THREE.TextureLoader`, `GLTFLoader` from `three/addons`
- Show loading progress via callbacks to UI

```js
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';

const loader = new GLTFLoader();

function loadModel(path) {
  return new Promise((resolve, reject) => {
    loader.load(
      path,
      (gltf) => resolve(gltf.scene),
      undefined,
      (error) => reject(error),
    );
  });
}
```

## Input Handling (Mobile-First)

All games MUST work on desktop AND mobile unless explicitly specified otherwise. Allocate 60% effort to mobile / 40% desktop when making tradeoffs.

| Game Type | Primary Mobile Input | Fallback |
|-----------|---------------------|----------|
| Marble/tilt/balance | Gyroscope (DeviceOrientation) | Virtual joystick |
| Runner/endless | Tap zones (left/right half) | Swipe gestures |
| Puzzle/turn-based | Tap targets (44px min) | Drag & drop |
| Shooter/aim | Virtual joystick + tap-to-fire | Dual joysticks |
| Platformer | Virtual D-pad + jump button | Tilt for movement |

Use a dedicated InputSystem that merges keyboard, gyroscope, and touch into a single analog interface. Game logic reads `moveX`/`moveZ` (-1..1) and never knows the source. Keyboard input is always active as an override; on mobile, the system initializes gyroscope (with iOS 13+ permission request) or falls back to a virtual joystick.

## When Adding Features

1. Create a new module in the appropriate `src/` subdirectory
2. Define new events in `EventBus.js` Events object using `domain:action` naming
3. Add configuration to `Constants.js`
4. Add state to `GameState.js` if needed
5. Wire it up in `Game.js` orchestrator
6. Communicate with other systems ONLY through EventBus

## Pre-Ship Validation Checklist

- [ ] **Core loop works** — Player can start, play, lose/win, and see the result
- [ ] **Restart works cleanly** — `GameState.reset()` restores a clean slate, all Three.js resources disposed
- [ ] **Touch + keyboard input** — Game works on mobile (gyro/joystick/tap) and desktop (keyboard/mouse)
- [ ] **Responsive canvas** — Renderer resizes on window resize, camera aspect updated
- [ ] **All values in Constants** — Zero hardcoded magic numbers in game logic
- [ ] **EventBus only** — No direct cross-module imports for communication
- [ ] **Resource cleanup** — Geometries, materials, textures disposed when removed from scene
- [ ] **No postprocessing** — Unless explicitly needed and tested on mobile
- [ ] **Shadows disabled** — Unless explicitly needed and budget allows
- [ ] **Delta-capped movement** — `Math.min(clock.getDelta(), 0.1)` on every frame
- [ ] **Build passes** — `npm run build` succeeds with no errors
- [ ] **No console errors** — Game runs without uncaught exceptions or WebGL failures

## Note on this adapted copy

Adapted from the open-source `game-creator` plugin (PlayableIntelligence/game-creator, MIT licensed) for this repo. Companion reference files (`core-patterns.md` with full EventBus/GameState/Constants code, `tsl-guide.md`, `input-patterns.md`, and the `threejs-perf` skill) were not bundled. Play.fun-specific safe-zone/monetization guidance from the original has been removed since it doesn't apply here.
```

- [ ] **Step 2: Validate frontmatter parses**

Run:
```bash
python3 -c "
import re, yaml
content = open('.agents/skills/threejs-game/SKILL.md').read()
fm = re.match(r'^---\n(.*?)\n---\n', content, re.DOTALL).group(1)
data = yaml.safe_load(fm)
assert data['name'] == 'threejs-game'
assert 'description' in data
print('OK:', data['name'])
"
```
Expected: `OK: threejs-game`

- [ ] **Step 3: Commit**

```bash
git add .agents/skills/threejs-game/SKILL.md
git commit -m "Add generic threejs-game skill"
```

---

### Task 4: Write `game-client-patterns.md` (the JoinQuest-specific mapping layer)

**Files:**
- Create: `.agents/skills/joinquest-integration/game-client-patterns.md`

**Interfaces:**
- Consumes: concepts defined in `game-architecture` (Task 1) — server-authoritative state, client/server split, disconnect/reconnect grace periods.
- Consumes: JoinQuest API contract facts from `docs/developer-agent-playbook.md` Phase 3 (`healthz`/`status`/`game-modes`/`matches`/`claim`) and `docs/reference-games.md`.
- Produces: a file referenced by name from `docs/developer-agent-playbook.md` Phase 9 (Task 5) and from `.agents/skills/joinquest-integration/SKILL.md`'s phase table (Task 6).

- [ ] **Step 1: Create the file**

```markdown
# JoinQuest game-client patterns

How the generic multiplayer patterns in the `game-architecture` skill map onto JoinQuest's actual integration contract. Read `game-architecture`, `phaser`, and `threejs-game` first for the underlying principles — this file is the thin JoinQuest-specific layer on top.

## Server-authoritative state ↔ your game API

`game-architecture`'s rule — match state must be server-authoritative — is not optional on JoinQuest, it's structural: JoinQuest itself only holds seat assignment and match provisioning, not gameplay state. Your game's own backend (the one exposing `healthz`/`status`/`game-modes`/`matches`/`claim`) is where gameplay state must live. There is no other server in this picture — if your game state only lives in the browser, there is nothing to resync a reconnecting player from, and nothing preventing a player from editing client state to affect the match.

## Disconnect / reconnect ↔ seat claim

The generic pattern (grace period, server-authoritative resync, defined expiry behavior) maps onto JoinQuest's JWT seat-claim flow directly:

- A player's seat is claimed via `POST /api/v1/matches/{externalMatchId}/claim` with their seat JWT. A **reconnect is just another claim** with the same JWT (until it expires) — your game API should treat "player reconnected" and "player claimed their seat for the first time" as the same code path, not two separate ones.
- During your grace period, keep the seat reserved in your own game state — JoinQuest doesn't need to be told about a brief disconnect; that's entirely your game backend's concern, not a JoinQuest API call.
- If your grace period expires and you forfeit the player or end the match, that's also purely your game's own logic — JoinQuest doesn't have a "forfeit" concept; the match is yours until you return the player to JoinQuest via the return path.
- If `seatTemplate`/game-modes changed while working on this (e.g. you added a spectator mode for disconnected-but-reconnecting players), call `joinquest_integration_sync_game_manifest` before your next integration check run — JoinQuest caches your manifest and won't see the change otherwise.

## No pause menu ↔ the room/table model

Because a JoinQuest match is played by real people who joined via a room/table together, the "don't freeze the game for everyone" rule from `game-architecture` has a concrete social dimension here: the other players in that room can see who's in the table. A silent freeze reads as the game being broken, not paused. If you do need to communicate a disconnect state, surface it in-game ("Waiting for Alex to reconnect...") rather than leaving other players guessing.

## Return path

`game-architecture` doesn't cover this because it's entirely JoinQuest-specific: after a match ends (win/lose/draw, or a forfeited disconnect), use the same `lobbyReturn` / match-result-callback pattern documented in [developer-integration-guide.md](../../../docs/developer-integration-guide.md) §8 and demonstrated in both reference games. This is the one piece of "what happens after the core loop ends" that has no generic multiplayer-game equivalent — it's unique to being hosted on JoinQuest.

## Reference games, reframed

[reference-games.md](../../../docs/reference-games.md) lists `rpslr` (duel, simplest case) and `wordhunt` (party/multi-seat, richer sync) — read them now specifically for how they implement the patterns above: seat claim as reconnect path, where match state lives (their game server, not the client), and their return-to-JoinQuest flow.
```

- [ ] **Step 2: Verify all internal links resolve**

Run:
```bash
cd .agents/skills/joinquest-integration
test -f ../../../docs/developer-integration-guide.md && echo "integration-guide: OK"
test -f ../../../docs/reference-games.md && echo "reference-games: OK"
```
Expected: both lines print `OK`.

- [ ] **Step 3: Commit**

```bash
cd /path/to/lobby
git add .agents/skills/joinquest-integration/game-client-patterns.md
git commit -m "Add game-client-patterns.md: JoinQuest-specific mapping onto generic multiplayer patterns"
```

---

### Task 5: Rewrite Phase 9 in the canonical playbook

**Files:**
- Modify: `docs/developer-agent-playbook.md:295-324` (the `## Phase 9` section)

**Interfaces:**
- Consumes: skill names from Tasks 1–4 (`game-architecture`, `phaser`, `threejs-game`, `game-client-patterns.md`).
- Produces: updated canonical content that Task 7's sync step propagates to three embedded copies.

- [ ] **Step 1: Replace the Phase 9 section**

Find the section starting at `## Phase 9 — Build the playable game (beyond the handshake)` and ending right before `## MCP tool quick reference`. Replace it with:

```markdown
## Phase 9 — Build the playable game (beyond the handshake)

**Goal:** Players get an **immersive, fun session** at the launch URLs — not just a stub that passes checks.

JoinQuest integration (Phases 3–5) proves the **wire contract**. Phase 9 is everything that makes people want to come back: rules, feel, clarity, pacing, and polish in **your game repo** (client + any game-specific server logic).

**When to start:** As soon as the API skeleton exists (Phase 3), sketch the client in parallel. After checks are green (Phase 5), shift focus here before asking for public release.

**Use the game-building skills, not just this playbook.** Phase 9 used to be "read the reference games and figure it out" — that's no longer the whole story. Three skills now cover the actual architecture work:

| Skill | Covers |
|-------|--------|
| `game-architecture` | Generic multiplayer patterns: client/server split (what must be server-authoritative), session continuity (disconnect/reconnect, grace periods, resync) — applies to any multiplayer game, not JoinQuest-specific |
| `phaser` / `threejs-game` | Engine-specific implementation patterns (scene/EventBus/GameState architecture, performance, mobile input) for 2D or 3D respectively |
| `game-client-patterns` (this skill folder — see [game-client-patterns.md](../.agents/skills/joinquest-integration/game-client-patterns.md)) | The thin JoinQuest-specific layer: how those generic patterns map onto your game's API contract, the seat-claim reconnect flow, and the return-to-JoinQuest path |

Read `game-architecture` and the relevant engine skill for the *how*, then `game-client-patterns.md` for how it plugs into JoinQuest specifically.

**Do not build a pause menu.** This used to not be called out and it's a common mistake — see `game-architecture`'s session-continuity section for what replaces it.

**Reference games** — read or clone the closest match ([reference-games.md](./reference-games.md)):

| If the game is… | Start with | Why |
|-----------------|------------|-----|
| 1v1, quick rounds, first integration | [rpslr](https://github.com/ScruffyProdigy/rpslr) | Minimal duel API + client + tests |
| Party / multi-seat, richer UI | [wordhunt](https://github.com/ScruffyProdigy/wordhunt) | Path launch URLs, sync, party flow |

Play live first when you can: [rpsls-duel.win](https://rpsls-duel.win), [word-hunt-arena.win](https://word-hunt-arena.win).

**Agent role (stay in the game repo):**

1. **Boot from JoinQuest** — client reads `token` (and match/seat from URL); claim or validate seat JWT; show who you're playing with.
2. **Core loop** — implement the rules and interactions from Phase 1; keep sessions short enough for "one more round." Follow `game-architecture`'s core-loop-first principle: input → action → fail condition → scoring → restart, before any polish.
3. **Multiplayer feel** — realtime or turn sync; clear whose turn; disconnect/reconnect per `game-architecture` and `game-client-patterns.md` (not a pause menu); win/lose/draw states.
4. **Return path** — "Back to JoinQuest" using the same patterns as reference games (`lobbyReturn`, match result callbacks per integration guide §8).
5. **Playtest loop** — developer runs a test table (Phase 7); fix confusion and bugs; **update catalog copy** if the built game differs from Phase 1 drafts before Phase 6/8.
6. **Polish when asked** — sound, animation, copy tweaks — after the loop works; don't block integration on polish.

**Do not** stop helping after checks pass. **Do** separate JoinQuest dashboard ops (MCP) from game implementation (game repo). If the developer pivots to gameplay, follow their lead while keeping launch URLs and handoff working.

**Done when:** Developer confirms friends enjoyed a real session (Phase 7) and the experience matches the catalog promise (or metadata was updated to match).
```

- [ ] **Step 2: Update the "Build game" row in the MCP tool quick reference table**

Find this line further down in the same file:

```
| Build game | Reference repos — [reference-games.md](./reference-games.md); no MCP substitute for gameplay code |
```

Replace with:

```
| Build game | `game-architecture`, `phaser`/`threejs-game`, `game-client-patterns.md`, [reference-games.md](./reference-games.md); no MCP substitute for gameplay code |
```

- [ ] **Step 3: Update the Phase 9 line in the phase checklist at the bottom of the file**

Find:

```
[ ] Phase 9 — Playable game client (immersive loop; metadata matches reality)
```

Replace with:

```
[ ] Phase 9 — Playable game client, built with game-architecture/phaser/threejs-game + game-client-patterns.md (immersive loop; metadata matches reality)
```

- [ ] **Step 4: Verify the edits landed**

Run:
```bash
grep -c "game-architecture" docs/developer-agent-playbook.md
```
Expected: `3` (one in the skills table, one in agent-role step 3, one in the phase checklist line) — plus one more in the MCP tool quick reference row, so actually expect `4`. Confirm the count printed matches what you just added; if it's lower, one of the replacements didn't land — check for typos in the match strings above.

- [ ] **Step 5: Commit**

```bash
git add docs/developer-agent-playbook.md
git commit -m "Rewrite Phase 9 to point developers at the new game-building skills"
```

---

### Task 6: Propagate the playbook change and update `SKILL.md`'s phase table

**Files:**
- Modify: `.agents/skills/joinquest-integration/SKILL.md:43` (the Phase 9 row in the phases table)
- Generated (do not hand-edit): `.agents/skills/joinquest-integration/playbook.md`, `backend/internal/developer/agent_playbook.md`, `plugins/joinquest/skills/joinquest-integration/playbook.md`

**Interfaces:**
- Consumes: Task 5's canonical playbook edit.

- [ ] **Step 1: Run the sync script**

```bash
./scripts/sync-developer-docs.sh
```
Expected output includes:
```
Synced docs/developer-agent-playbook.md -> backend/internal/developer/agent_playbook.md
Synced docs/developer-agent-playbook.md -> .agents/skills/joinquest-integration/playbook.md
Synced agent skill -> plugins/joinquest/skills/joinquest-integration/
```

- [ ] **Step 2: Verify the embedded copy picked up the change**

```bash
grep -q "game-architecture" .agents/skills/joinquest-integration/playbook.md && echo "embedded copy: updated"
```
Expected: `embedded copy: updated`

- [ ] **Step 3: Update the phases summary table in `SKILL.md`**

In `.agents/skills/joinquest-integration/SKILL.md`, find this row in the phases table:

```
| 9 | Build playable game | Client + core loop at launch URLs; see playbook Phase 9 + [reference games](https://github.com/ScruffyProdigy/playhub/blob/main/docs/reference-games.md) |
```

Replace with:

```
| 9 | Build playable game | Client + core loop at launch URLs; see playbook Phase 9, the `game-architecture`/`phaser`/`threejs-game` skills, and [game-client-patterns.md](./game-client-patterns.md) |
```

- [ ] **Step 4: Verify and commit**

```bash
grep -q "game-client-patterns.md" .agents/skills/joinquest-integration/SKILL.md && echo "SKILL.md: updated"
git add .agents/skills/joinquest-integration/SKILL.md .agents/skills/joinquest-integration/playbook.md backend/internal/developer/agent_playbook.md plugins/joinquest/skills/joinquest-integration/playbook.md
git commit -m "Sync Phase 9 rewrite to embedded playbook copies; update SKILL.md phase table"
```

---

### Task 7: Extend the sync script and plugin manifest to ship the three new skills

**Files:**
- Modify: `scripts/sync-developer-docs.sh`
- Modify: `plugins/joinquest/.claude-plugin/plugin.json`
- Modify: `plugins/joinquest/.cursor-plugin/plugin.json`

**Interfaces:**
- Consumes: `.agents/skills/game-architecture/`, `.agents/skills/phaser/`, `.agents/skills/threejs-game/` (Tasks 1–3) as sync sources.
- Produces: `plugins/joinquest/skills/game-architecture/`, `plugins/joinquest/skills/phaser/`, `plugins/joinquest/skills/threejs-game/` as sync targets, matching how `joinquest-integration` is already packaged.

- [ ] **Step 1: Add the three new skill folders to the sync script's plugin-copy step**

In `scripts/sync-developer-docs.sh`, find:

```bash
PLUGIN_SKILL="$ROOT/plugins/joinquest/skills/joinquest-integration"
mkdir -p "$PLUGIN_SKILL"
cp "$ROOT/.agents/skills/joinquest-integration/SKILL.md" "$PLUGIN_SKILL/"
cp "$ROOT/.agents/skills/joinquest-integration/mcp-setup.md" "$PLUGIN_SKILL/"
cp "$ROOT/.agents/skills/joinquest-integration/playbook.md" "$PLUGIN_SKILL/"
echo "Synced agent skill -> plugins/joinquest/skills/joinquest-integration/"
```

Replace with:

```bash
PLUGIN_SKILL="$ROOT/plugins/joinquest/skills/joinquest-integration"
mkdir -p "$PLUGIN_SKILL"
cp "$ROOT/.agents/skills/joinquest-integration/SKILL.md" "$PLUGIN_SKILL/"
cp "$ROOT/.agents/skills/joinquest-integration/mcp-setup.md" "$PLUGIN_SKILL/"
cp "$ROOT/.agents/skills/joinquest-integration/playbook.md" "$PLUGIN_SKILL/"
cp "$ROOT/.agents/skills/joinquest-integration/game-client-patterns.md" "$PLUGIN_SKILL/"
echo "Synced agent skill -> plugins/joinquest/skills/joinquest-integration/"

for skill in game-architecture phaser threejs-game; do
  TARGET="$ROOT/plugins/joinquest/skills/$skill"
  mkdir -p "$TARGET"
  cp "$ROOT/.agents/skills/$skill/SKILL.md" "$TARGET/"
  echo "Synced agent skill -> plugins/joinquest/skills/$skill/"
done
```

- [ ] **Step 2: Update the plugin manifests' description and version**

In `plugins/joinquest/.claude-plugin/plugin.json`, change:

```json
  "description": "JoinQuest agent skill and MCP — integrate multiplayer games with joinquest.cc",
  "version": "1.0.0",
```

to:

```json
  "description": "JoinQuest agent skill and MCP — integrate multiplayer games with joinquest.cc, plus generic multiplayer game-architecture skills (Phaser, Three.js) for building the game itself",
  "version": "1.1.0",
```

Make the identical change in `plugins/joinquest/.cursor-plugin/plugin.json`.

- [ ] **Step 3: Run the sync script and verify the new skill folders appear**

```bash
./scripts/sync-developer-docs.sh
ls plugins/joinquest/skills/
```
Expected: `game-architecture  joinquest-integration  phaser  threejs-game`

- [ ] **Step 4: Verify plugin JSON is still valid**

```bash
python3 -c "import json; json.load(open('plugins/joinquest/.claude-plugin/plugin.json')); print('valid')"
python3 -c "import json; json.load(open('plugins/joinquest/.cursor-plugin/plugin.json')); print('valid')"
```
Expected: `valid` printed twice.

- [ ] **Step 5: Commit**

```bash
git add scripts/sync-developer-docs.sh plugins/joinquest/
git commit -m "Ship game-architecture, phaser, threejs-game skills as part of the JoinQuest plugin"
```

---

### Task 8: Re-package the personal `.skill` uploads with final content

**Files:**
- No repo files — this task produces `.skill` zip files for manual Cowork upload (Ryan's personal environment), since the marketplace-add flow was broken when this work started.

**Interfaces:**
- Consumes: final `.agents/skills/game-architecture/SKILL.md`, `.agents/skills/phaser/SKILL.md`, `.agents/skills/threejs-game/SKILL.md` from Tasks 1–3 (identical content, repo copy is now the source of truth).

- [ ] **Step 1: Build the three `.skill` zips from the repo copies**

```bash
cd /path/to/lobby
mkdir -p /tmp/skill-export
for skill in game-architecture phaser threejs-game; do
  rm -rf "/tmp/skill-export/$skill"
  mkdir -p "/tmp/skill-export/$skill"
  cp ".agents/skills/$skill/SKILL.md" "/tmp/skill-export/$skill/SKILL.md"
done
python3 -c "
import zipfile, os
for skill in ['game-architecture', 'phaser', 'threejs-game']:
    src = f'/tmp/skill-export/{skill}'
    zpath = f'/tmp/skill-export/{skill}.skill'
    with zipfile.ZipFile(zpath, 'w', zipfile.ZIP_DEFLATED) as zf:
        for root, dirs, files in os.walk(src):
            for f in files:
                full = os.path.join(root, f)
                arcname = os.path.relpath(full, '/tmp/skill-export')
                zf.write(full, arcname)
    print(zpath, os.path.getsize(zpath), 'bytes')
"
```
Expected: three lines, one per skill, each with a nonzero byte count.

- [ ] **Step 2: Hand off for manual upload**

These three `.skill` files replace the ones uploaded earlier this session (before the `game-architecture` client/server-split content existed). Upload via Settings → Skills in Cowork, same as before. No commit — this step doesn't touch the repo.

---

### Task 9: Track this work in Linear

**Files:** None (Linear issues only).

**Interfaces:**
- Consumes: Tasks 1–8 as the source list for issue titles/descriptions.

- [ ] **Step 1: Create a parent issue in the JoinQuest Linear project**

Use the Linear MCP tool to create an issue titled "Game design skills for JoinQuest developers" in the `JoinQuest Platform` project, with description linking to `docs/superpowers/specs/2026-08-05-game-design-skills-design.md` and `docs/superpowers/plans/2026-08-05-game-design-skills.md`.

- [ ] **Step 2: Create one sub-issue per task**

Create 8 sub-issues (Tasks 1–8 above), each titled after its task heading (e.g. "Add the game-architecture skill to the repo"), parented to the issue from Step 1.

- [ ] **Step 3: Verify**

Read back the created issues via the Linear MCP tool and confirm the parent/child relationship is correct and all 8 sub-issues are present.

---

### Task 10: Add a Notion page for visibility

**Files:** None (Notion page only).

**Interfaces:**
- Consumes: the design spec and this plan.

- [ ] **Step 1: Create a Notion page**

Create a page titled "Spec: Game Design Skills for Developers" in the same location as the existing "Spec: Mode & Queue Eligibility" page, matching its style. Content: a short summary of the problem (Phase 9 had no game-building guidance), the decision (generic skills + one JoinQuest-specific mapping doc, no premature platform-specific skill names), and links to the spec, the plan, and the Linear parent issue.

- [ ] **Step 2: Link it from the Roadmap page**

Add a line linking the new spec page, consistent with how other specs are referenced from Roadmap.

- [ ] **Step 3: Verify**

Read back the created page via the Notion MCP tool and confirm the title and links are correct.

---

## Plan self-review notes

- **Spec coverage:** Non-goals (no engine tutorial duplication, no contract duplication) — respected, Tasks 1–3 don't touch integration-contract content. Naming rule (platform-specific gets prefixed / lives in `joinquest-integration`) — respected, only `game-client-patterns.md` and the Phase 9 rewrite touch JoinQuest-specific content. Rollout order from the spec — followed 1:1 by task order, with Linear/Notion tasks appended per the later scope decision to track the plan (not just the spec) there.
- **Placeholder scan:** none found — every task has real, complete file content or a real command.
- **Type/name consistency:** skill names (`game-architecture`, `phaser`, `threejs-game`, `game-client-patterns.md`) are spelled identically across Tasks 1–7; verified by the `grep` checks in Tasks 5–7.
- **Open item carried from spec, not resolved here:** whether `game-client-patterns.md` should also be MCP-servable (spec's open question) — out of scope for this plan; would be a follow-up plan if decided.
