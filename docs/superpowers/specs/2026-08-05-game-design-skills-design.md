# Game design skills for JoinQuest developers — design

**Status:** Draft
**Author:** Ryan Kohler (with Claude)
**Date:** 2026-08-05

## Problem

`joinquest-integration` (the skill/MCP bundle developers install to build on JoinQuest) covers the platform contract well: registration, `healthz`/`status`/`game-modes`, provisioning, JWT claim, integration checks, public release. Phase 9 of that skill ("Build playable game") is the last step before a developer has a real product, and today it's a single line pointing at [`docs/reference-games.md`](../../reference-games.md) — two example repos and nothing else. There's no guidance on how to actually architect a good multiplayer game once the contract is wired up.

We validated this gap directly: while adapting the open-source `game-creator` plugin (Phaser/Three.js scaffolding skills) for personal use, its game-flow guidance included a "pause menu" step lifted from single-player game design, which is actively wrong for JoinQuest's server-authoritative multiplayer model (pausing either freezes the game for other players or desyncs the pausing client). That correction — replace pause with a non-blocking settings overlay or turn timers, and add real disconnect/reconnect session-continuity guidance — is the seed of what this spec formalizes.

## Goal

Give developers (and their coding agents) real architectural guidance for the part of game-building that isn't platform integration: what belongs in the client vs. the backend, how to handle multiplayer session continuity, and adjacent topics — delivered as skills that trigger contextually as the developer works on different parts of their game, rather than one monolithic document.

## Non-goals

- Not building a full game-engine tutorial (Phaser/Three.js mechanics are already covered by the existing `phaser` and `threejs-game` skills, adapted from `game-creator` and kept generic/unbranded).
- Not duplicating the integration contract — `joinquest-integration` already owns registration, checks, and the API contract itself.
- Not inventing new "JoinQuest-branded" skill identities for content that is actually generic multiplayer game design knowledge. See Naming rule below.

## Naming rule

**Anything platform-specific gets a `joinquest-` prefix (or lives inside the `joinquest-integration` bundle). Everything else stays generic and unbranded**, because most of what a multiplayer game needs architecturally (server-authoritative state, client/server split, disconnect/reconnect) applies to any multiplayer game, not just ones built on JoinQuest. Skill `name` fields are short identifiers, not descriptions — the `description` frontmatter field is what drives auto-triggering, so names don't need to be self-explanatory sentences.

## Design

**Note:** the structure below reflects what we've identified so far. It's plausible that once we're actually writing content for `game-client-patterns.md`, we'll find genuinely platform-specific game-building concerns (not just contract-mapping) that warrant their own properly-prefixed skill rather than a section in an existing one. The naming rule above still governs if/when that happens — this design isn't a ceiling on new skills, it's a statement that we haven't found the need for one yet.

### 1. `game-architecture` skill (existing, expand)

Already adapted from `game-creator` (MIT-licensed, stripped of Play.fun-specific content) and already amended with a disconnect/reconnect section replacing the single-player "pause menu" flow. This spec extends it further with a generic **client/server split** section, since that turns out to be universal multiplayer architecture, not JoinQuest-specific:

- What must be server-authoritative in any real-time or turn-based multiplayer game (match state, scoring, win/loss determination, anything that must survive a reconnect) vs. what's safely client-only (rendering, input handling, local prediction/juice).
- Common failure modes: trusting client-reported state for anything that affects other players; recomputing authoritative state from client input without validation; state that only exists in one client's memory.
- How this interacts with the session-continuity section already present (reconnect requires a server to resync *from*).

No JoinQuest branding. Useful to any developer building a multiplayer browser game, on this platform or not.

### 2. `phaser`, `threejs-game` skills (existing, unchanged)

Stay as-is — engine-specific, generic, not part of this spec's scope beyond what's already shipped.

### 3. `joinquest-integration` skill (existing, extend)

Two changes to the existing bundle (`SKILL.md`, `playbook.md`, `mcp-setup.md`):

- **Rewrite Phase 9** in `SKILL.md` and `playbook.md` to explicitly flow the developer/agent into `game-architecture`, `phaser`, and `threejs-game` for the "how do I build a good game" question, instead of only pointing at reference repos.
- **Add a new companion file**, `game-client-patterns.md`, alongside `playbook.md` and `mcp-setup.md`. This is the only place in the whole design that is legitimately platform-specific: it's the mapping layer between the generic concepts in `game-architecture` and JoinQuest's actual contract. Examples of what belongs here:
  - "Your server-authoritative match state is what backs the `matches` provision endpoint and the JWT `claim` flow."
  - "Reconnect logic should call `joinquest_integration_sync_game_manifest` after a `seatTemplate` change, and re-resolve the player's seat via a fresh claim, not a cached one."
  - "The room/table model means a disconnect grace period should be visible to other players in the same room, not just logged silently."
  - Reference-game pointers stay here too (`rpslr` for the duel/session-simple case, `wordhunt` for realtime-sync/party case), now framed as "here's where these generic patterns show up in a real repo" rather than the only guidance offered.

### What we're explicitly not building yet

A `game-qa`-equivalent skill (Playwright/gameplay testing) was discussed and set aside — `game-creator`'s own `game-qa` skill already covers generic gameplay QA reasonably well, and JoinQuest's own integration tests (integration guide §8) are already documented. Revisit only if a real, JoinQuest-specific testing gap turns up once developers actually start using this.

## Rollout order

1. Finish expanding `game-architecture` with the client/server split section (this spec's primary content work).
2. Write `game-client-patterns.md` and wire it into `joinquest-integration`.
3. Rewrite Phase 9 in `SKILL.md` and `playbook.md`.
4. Re-package the personal `.skill` uploads (`phaser`, `threejs-game`, `game-architecture`) to pick up the changes, since they were manually installed outside the plugin marketplace due to the marketplace-sync issue encountered earlier.
5. Add a short Notion page pointing at this spec, consistent with the existing "Spec: Mode & Queue Eligibility" page style, for visibility alongside the Roadmap.

## Open questions

- Should `game-client-patterns.md` be discoverable via MCP (like `joinquest_integration_get_integration_guide`) so agents without local repo access can pull it, or is bundling it in the skill folder (read directly from the repo) sufficient for v1?
- Do the two existing reference games (`rpslr`, `wordhunt`) actually demonstrate the disconnect/reconnect pattern today, or does this spec imply updating them too?
- **TBD, deferred until content work starts:** are there platform-specific *game design* concerns (not integration-contract mapping) that deserve their own `joinquest-`-prefixed skill? Candidates to watch for while writing `game-client-patterns.md`: anything shaped by the room/table model, seat-based (vs. free-for-all) matchmaking, or launch-URL-driven session boot that doesn't reduce to "here's the generic pattern, here's the endpoint." If real content of that shape piles up, split it out rather than overloading the mapping doc.
