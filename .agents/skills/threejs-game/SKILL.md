---
name: threejs-game
description: >-
  Build 3D browser games with Three.js using event-driven modular architecture. Use when
  creating a new 3D game, adding 3D game features, setting up Three.js scenes, or working
  on any Three.js game project. Covers engine mechanics only — for multiplayer architecture see
  the game-architecture and multiplayer-game-design skills.
---

# Three.js Game Development

You are an expert Three.js game developer. Follow these opinionated patterns when building 3D browser games.

**Scope.** This skill covers Three.js engine mechanics, and its defaults assume a single local player. Guidance that changes once other real players share the match is flagged inline with a **Multiplayer note**. Read `game-architecture` (code structure, state, prediction/reconciliation) and `multiplayer-game-design` (server authority, disconnect/reconnect, turn structure, no pause menu) alongside this file for anything with more than one player in it.

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

**Multiplayer note:** `reset()` alone doesn't cover it. Match end is signalled by the server rather than triggered by a local restart button, and client-side prediction needs a cheap clone/restore path so state can be rewound during reconciliation — see `game-architecture`.

## Core Patterns (Non-Negotiable)

### 1. EventBus Singleton
ALL inter-module communication goes through an EventBus (`core/EventBus.js`). Modules never import each other directly for communication. Provides `on`, `once`, `off`, `emit`, and `clear` methods. Events use `domain:action` naming (e.g., `player:hit`, `game:over`).

### 2. Centralized GameState
One singleton (`core/GameState.js`) holds ALL game state. Systems read from it, events update it. Must include a `reset()` method that restores a clean slate for restarts.

**Multiplayer note:** state becomes keyed by player ID, and predicted state has to stay separable from server-confirmed state. See `game-architecture`.

### 3. Constants File
Every magic number, balance value, asset path, and configuration goes in `core/Constants.js`. Never hardcode values in game logic. Organize by domain: `PLAYER_CONFIG`, `ENEMY_CONFIG`, `WORLD`, `CAMERA`, `COLORS`, `ASSET_PATHS`.

### 4. Game.js Orchestrator
The Game class (`core/Game.js`) initializes everything and runs the render loop. Uses `renderer.setAnimationLoop()` — the official Three.js pattern (handles WebGPU async correctly and pauses when the tab is hidden). Sets up renderer, scene, camera, systems, UI, and event listeners in `init()`.

**Multiplayer note:** the automatic pause on hidden tabs is a benefit in single-player and a hazard in multiplayer — the server keeps simulating while the loop is stopped, so the player returns to a stale world. Treat `visibilitychange` as a session event needing an explicit policy (resync from a full server snapshot on resume, and possibly a disconnect grace period), not as free battery savings. See `multiplayer-game-design`.

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

- **Use `renderer.setAnimationLoop()`** instead of manual `requestAnimationFrame`. It pauses when the tab is hidden and handles WebGPU async correctly — in multiplayer that pause needs an explicit resync policy, per the orchestrator note above.
- **Cap delta time**: `Math.min(clock.getDelta(), 0.1)` to prevent death spirals. **Multiplayer note:** a variable delta makes the simulation non-deterministic, which breaks the predict-and-rewind pattern in `game-architecture` — real-time netcode needs a fixed timestep for the simulation step, with rendering interpolated separately
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
- [ ] **Multiplayer?** — If the game has more than one real player, also run the checklists in `game-architecture` and `multiplayer-game-design`. Two items above change meaning: "restart works cleanly" becomes a synchronized, server-signalled match end, and "core loop works" is judged per match rather than per client

## Note on this adapted copy

Adapted from the open-source `game-creator` plugin (PlayableIntelligence/game-creator, MIT licensed) for this repo. Companion reference files (`core-patterns.md` with full EventBus/GameState/Constants code, `tsl-guide.md`, `input-patterns.md`, and the `threejs-perf` skill) were not bundled. Play.fun-specific safe-zone/monetization guidance from the original has been removed since it doesn't apply here.
