---
name: joinquest-integration
description: >-
  Walks a developer through integrating a multiplayer game with JoinQuest — open-ended
  game discovery, registration fields, MCP setup, game API implementation (healthz, provision,
  JWT), integration checks, catalog metadata, test tables, public release, and building the
  playable game client after the handshake. Use when the user mentions JoinQuest, joinquest.cc,
  lobby integration, matchmaking, game registration, hosting a game on JoinQuest, or integrating
  with the JoinQuest MCP server.
---

# JoinQuest integration

You are helping a developer integrate their **multiplayer game** with [JoinQuest](https://joinquest.cc). Follow the phases below in order. Read `playbook.md` in this skill folder for full detail, or call MCP `joinquest_integration_get_agent_playbook` for the latest copy from JoinQuest.

## Before you start

- Load **playbook.md** (bundled) or MCP `joinquest_integration_get_agent_playbook`.
- If JoinQuest MCP is configured, use MCP tools for all dashboard operations. If not, guide Phase 2 setup first (`mcp-setup.md`).
- For API/provision/JWT details, call MCP `joinquest_integration_get_integration_guide` — do not guess wire formats.
- After implementing the game API, add **local tests** per integration guide §8 ([rpslr](https://github.com/ScruffyProdigy/rpslr) reference). Run `npm test` before remote checks.
- After checks pass, continue in the **game repo** for client/gameplay (Phase 9) — integration is not the finish line.

## Hard rules

1. **Interview before implementing** (Phase 1). Start open-ended — invite the developer to describe their game in their own words, then ask clarifying questions only for what's still unclear. Confirm reasonable inferences ("It sounds like a 2-player game — fair?") instead of guessing or re-asking from scratch.
2. **Never** call `joinquest_integration_register_game`, `joinquest_integration_update_game_metadata`, `joinquest_integration_connect_game` (when changing URL), `joinquest_integration_rotate_webhook_secret`, or `joinquest_integration_request_public_release` without explicit human approval.
3. **No localhost** for `apiBaseUrl` — JoinQuest must reach a public HTTPS URL.
4. Fix the **game API** when checks fail, then re-run checks.

## Phases (summary)

| # | Phase | Done when |
|---|-------|-----------|
| 1 | Discover game | Developer described the idea; drafts approved (copy, genre/difficulty, seatTemplate + socialMode plan) |
| 2 | Connect MCP | `joinquest_integration_list_my_games` works |
| 3 | Implement game API | healthz, status, game-modes, provision, claim on public HTTPS; **local tests** (integration guide §8) |
| 4 | Register on JoinQuest | Game registered via MCP or dashboard (`PRIVATE_TESTING` or draft) |
| 5 | Run checks | Required manifest + provision checks PASS |
| 6 | Save metadata | Human approved → `joinquest_integration_update_game_metadata` |
| 7 | Test with friends | Developer confirms test table / playtest — game is fun, not just wired |
| 8 | Public release | Human confirmed → `joinquest_integration_request_public_release` |
| 9 | Build playable game | Client + core loop at launch URLs; see playbook Phase 9 + [reference games](https://github.com/ScruffyProdigy/joinquest/blob/main/docs/reference-games.md) |

## MCP tool sequence (typical)

```
joinquest_integration_get_agent_playbook          # this workflow
joinquest_integration_get_discovery_prompt        # Phase 1 — open-ended prompt + follow-ups
joinquest_integration_get_catalog_tag_taxonomy    # genre, difficulty and socialMode IDs

# After MCP auth:
joinquest_integration_register_game               # Phase 4 — after developer confirms fields
joinquest_integration_connect_game                # Phase 4 — draft retry / apiBaseUrl change
joinquest_integration_list_my_games
joinquest_integration_get_game_credentials        # serviceToken (sensitive)
joinquest_integration_get_example_provision_payload
joinquest_integration_sync_game_manifest          # after seatTemplate/API fixes
joinquest_integration_run_game_checks             # repeat until green
joinquest_integration_update_game_metadata        # after human approval
joinquest_integration_request_public_release      # after human approval
```

## Registration fields (Phase 4)

Confirm with the developer, then call `joinquest_integration_register_game` or use https://joinquest.cc/developers.

| Field | Required |
|-------|----------|
| Game name | Yes |
| Slug | Yes |
| Short description | Yes |
| API base URL (public HTTPS) | Yes |
| Contact email | Yes |
| Website / community URL | No |

## On check failures

1. Read check `message` and `detail`.
2. MCP `joinquest_integration_get_integration_guide` → §11 checklist index.
3. Fix game API, deploy, `joinquest_integration_run_game_checks` again.

## Status template

Report progress using the checklist in `playbook.md` § Phase checklist.
