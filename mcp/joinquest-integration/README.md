# JoinQuest integration MCP server

stdio MCP server for **game developers** integrating with JoinQuest. Exposes developer dashboard operations to Cursor, Claude, and other agents via the same GraphQL API as the portal.

## Quick setup

1. Sign in at [JoinQuest](https://joinquest.cc) and open your **developer dashboard**.
2. **Connect an AI assistant** → **Show setup** → **Generate API key** (copy once).
3. Add MCP config (Node.js 20+, uses `npx` — no install step):

**Cursor** — `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "joinquest-integration": {
      "type": "stdio",
      "command": "npx",
      "args": ["--yes", "--package", "@joinquest/mcp-integration", "joinquest-integration-mcp-cursor"],
      "env": {
        "JOINQUEST_API_KEY": "lq_dev_..."
      }
    }
  }
}
```

**Claude Code:**

```bash
claude mcp add --scope project --transport stdio \
  --env JOINQUEST_API_KEY=lq_dev_... \
  joinquest-integration -- npx -y @joinquest/mcp-integration
```

**Other stdio clients** — use `npx` with args `["-y", "@joinquest/mcp-integration"]` and the same `env`.

4. Fully quit and reopen your agent client. Test: ask the agent to call `joinquest_integration_list_my_games`.

**Other clients** — GitHub Copilot, Roo Code, Windsurf, and Cline: use the matching tab on the [developer dashboard](https://joinquest.cc/developers) or from your game repo:

```bash
JOINQUEST_API_KEY=lq_dev_... curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/joinquest/main/scripts/install-joinquest-dev.sh | sh -s -- --copilot
```

Flags: `--copilot`, `--roo`, `--windsurf`, `--cline`. See [Client config paths](#client-config-paths) below.

**ChatGPT** is not supported (requires hosted HTTPS MCP). Use [Register in the browser](https://joinquest.cc/developers?path=manual) instead.

### Why a separate Cursor bin?

Cursor injects `ELECTRON_RUN_AS_NODE` into MCP child processes. Use the second npx arg `joinquest-integration-mcp-cursor` (wrapper that unsets it). Other clients use the default bin.

### Optional global install

```bash
curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/joinquest/main/scripts/install-joinquest-mcp.sh | sh
```

## Environment

| Variable | Required | Description |
|----------|----------|-------------|
| `JOINQUEST_API_KEY` | Yes* | From developer dashboard → Connect AI assistant |
| `JOINQUEST_ISSUER_URL` | No | Lobby JWT issuer / provision `lobbyId` |
| `JOINQUEST_PUBLIC_URL` | No | Browser Lobby URL for example provision payloads |
| `JOINQUEST_SESSION` | Legacy | Session cookie (prefer API key) |

Advanced (lobby contributors): `JOINQUEST_API_URL=http://localhost:8080/graphql` — see [docs/development.md](../../docs/development.md).

## Publishing

Maintainers: `./scripts/publish-joinquest-mcp.sh` (requires npm login + `@joinquest` scope). Roadmap: [docs/developer-ai-setup-roadmap.md](../../docs/developer-ai-setup-roadmap.md).

## Client config paths

| Client | Config file |
|--------|-------------|
| Cursor | `~/.cursor/mcp.json` or `.cursor/mcp.json` |
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Claude Code | `.mcp.json` at project root |
| GitHub Copilot | `.vscode/mcp.json` (`servers` key) |
| Roo Code | `.roo/mcp.json` at project root |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` (global) |
| Cline | VS Code `globalStorage/.../cline_mcp_settings.json` |
| ChatGPT | Not supported — requires hosted HTTPS MCP |

## Tools

| Tool | Purpose |
|------|---------|
| `joinquest_integration_get_agent_playbook` | **Start here** — end-to-end agent workflow (phases 1–8) |
| `joinquest_integration_get_integration_guide` | Full integration guide (markdown) — includes **§8 recommended local tests** for game repos |
| `joinquest_integration_get_discovery_prompt` | Open-ended discovery prompt + follow-up guide |
| `joinquest_integration_get_catalog_tag_taxonomy` | Valid tag IDs |
| `joinquest_integration_list_my_games` | Owner's games + visibility |
| `joinquest_integration_register_game` | Register a new game (confirm fields with developer first) |
| `joinquest_integration_get_game_checks` | Checklist + metadata |
| `joinquest_integration_run_game_checks` | Run manifest / provision / JWT suite (19 required checks; see integration guide §11) |
| `joinquest_integration_sync_game_manifest` | Re-fetch game-modes and refresh cached seats (after seatTemplate fixes) |
| `joinquest_integration_connect_game` | Connect / reconnect API; optional new `apiBaseUrl` (draft + private_testing) |
| `joinquest_integration_rotate_webhook_secret` | Rotate webhook secret (previous stops working) |
| `joinquest_integration_update_game_metadata` | Save catalog copy, name, contact/links, tags |
| `joinquest_integration_get_game_credentials` | serviceToken + webhook secret |
| `joinquest_integration_get_example_provision_payload` | Sample provision JSON |
| `joinquest_integration_request_public_release` | Submit for catalog review |

## Run from repo

```bash
cd mcp/joinquest-integration
npm install
JOINQUEST_API_KEY=your-key node src/index.js
```

## Tests

```bash
npm test
```

## Publishing (maintainers)

From repo root (requires `@joinquest` scope access + npm 2FA):

```bash
NPM_OTP=123456 ./scripts/publish-joinquest-mcp.sh
```

Use a code from your authenticator app when prompted. For GitHub Actions, add a **granular access token** with **Publish** on `@joinquest/*` and **bypass 2FA** enabled as `NPM_TOKEN`.
