---
name: phaser
description: >-
  Build 2D browser games with Phaser 3 using scene-based architecture and centralized state.
  Use when creating a new 2D game, adding 2D game features, working with Phaser, or building
  sprite-based web games. Covers engine mechanics only — for multiplayer architecture see the
  game-architecture and multiplayer-game-design skills.
---

# Phaser 3 Game Development

You are an expert Phaser game developer. Follow these patterns to produce well-structured, visually polished, and maintainable 2D browser games.

**Scope.** This skill covers Phaser engine mechanics, and its defaults assume a single local player. Guidance that changes once other real players share the match is flagged inline with a **Multiplayer note**. Read `game-architecture` (code structure, state, prediction/reconciliation) and `multiplayer-game-design` (server authority, disconnect/reconnect, turn structure, no pause menu) alongside this file for anything with more than one player in it.

## Core Principles

1. **Core loop first** — Implement the minimum gameplay loop before any polish: boot → preload → create → update. Add the win/lose condition and scoring **before** visuals, audio, or juice. Keep initial scope small: 1 scene, 1 mechanic, 1 fail condition.
2. **TypeScript-first** — Always use TypeScript for type safety and IDE support
3. **Scene-based architecture** — Each game screen is a Scene; keep them focused
4. **Vite bundling** — Use the official `phaserjs/template-vite-ts` template
5. **Composition over inheritance** — Prefer composing behaviors over deep class hierarchies
6. **Data-driven design** — Define levels, enemies, and configs in JSON/data files
7. **Event-driven communication** — All cross-scene/system communication via EventBus
8. **Restart-safe** — Gameplay must be fully restart-safe and deterministic. `GameState.reset()` must restore a clean slate. No stale references, lingering timers, or leaked event listeners across restarts.

**Multiplayer note:** `reset()` alone doesn't cover it. Match end is signalled by the server rather than triggered by a local restart button, and client-side prediction needs a cheap clone/restore path so state can be rewound during reconciliation — see `game-architecture`.

## Mandatory Conventions

All games MUST follow these conventions:

- **`core/` directory** with EventBus, GameState, and Constants
- **EventBus singleton** — `domain:action` event naming, no direct scene references
- **GameState singleton** — Centralized state with `reset()` for clean restarts. **Multiplayer note:** state becomes keyed by player ID, and predicted state has to stay separable from server-confirmed state
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
│   └── GameOver.ts        # End screen with restart (single-player shape — see note below)
├── objects/               # Game entities (Player, Enemy, etc.)
├── systems/               # Managers and subsystems
├── ui/                    # UI components (buttons, bars, dialogs)
├── audio/                 # Audio manager, music, SFX
├── config.ts              # Phaser.Types.Core.GameConfig
└── main.ts                # Entry point
```

**Multiplayer note:** this scene list is single-player-shaped. A multiplayer game needs a connect/seat-claim step before `Game` (you can't start playing until the server has you in a match), a waiting-for-players state, and a `Reconnecting` state — `multiplayer-game-design` treats reconnect UI as core loop, not polish. `GameOver` becomes a server-signalled match end rather than a scene the client decides to enter on its own.

## Scene Architecture

- **Lifecycle**: `init()` → `preload()` → `create()` → `update(time, delta)`
- Use `init()` for receiving data from scene transitions
- Load assets in a dedicated `Preloader` scene, not in every scene
- Keep `update()` lean — delegate to subsystems and game objects
- **No title screen by default** — boot directly into gameplay. Only add a title/menu scene if the user explicitly asks for one. **Multiplayer note:** "boot straight into gameplay" doesn't survive contact with a real match — connecting, claiming a seat, and waiting for opponents all have to happen first
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

**Multiplayer note:** Arcade and Matter both run client-side, so their results are not authoritative. Any collision, hit, or position that affects another player must be computed or revalidated server-side — a client's physics result is a claim, not a fact. See `multiplayer-game-design`.

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
- [ ] **Multiplayer?** — If the game has more than one real player, also run the checklists in `game-architecture` and `multiplayer-game-design`. Two items above change meaning: "restart works cleanly" becomes a synchronized, server-signalled match end, and "core loop works" is judged per match rather than per client

## Note on this adapted copy

Adapted from the open-source `game-creator` plugin (PlayableIntelligence/game-creator, MIT licensed) for this repo. Companion reference files (`conventions.md`, `project-setup.md`, `scenes-and-lifecycle.md`, `game-objects.md`, `physics-and-movement.md`, `assets-and-performance.md`, `patterns.md`, `no-asset-design.md`, and worked examples) that the original skill links to were not bundled — this file's content stands alone. References to the Play.fun SDK/safe-zone in the original have been removed since they don't apply outside that platform.
