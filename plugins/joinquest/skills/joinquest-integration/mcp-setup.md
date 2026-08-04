# JoinQuest MCP setup (for agents)

Guide the developer through this once per machine. Full UI copy lives on the developer dashboard → **Connect an AI assistant** (or `https://joinquest.cc/developers?path=ai`).

## Prerequisites

- Node.js 20+
- JoinQuest account (sign in at https://joinquest.cc/developers)

## Steps

1. **Generate API key** — Developer dashboard → Connect an AI assistant → **Show setup** → Generate API key. Copy once (`lq_dev_…`).

2. **Add MCP server** (uses `npx` — no install step). Pick the developer's editor:

### Cursor (recommended — plugin)

Install the JoinQuest Cursor plugin (skill + MCP in one step). From the [developer dashboard](https://joinquest.cc/developers) → **Connect an AI assistant** → Cursor tab, copy the install command with your API key filled in.

Or run manually:

```bash
export JOINQUEST_API_KEY=lq_dev_PASTE_HERE
curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/playhub/main/scripts/install-joinquest-cursor-plugin.sh | sh
```

Then **Cmd+Q** Cursor, reopen your game project, and start a fresh Agent chat.

Plugin source: [plugins/joinquest](https://github.com/scruffyprodigy/playhub/tree/main/plugins/joinquest)

**Manual MCP only** — paste into `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "joinquest-integration": {
      "type": "stdio",
      "command": "npx",
      "args": ["--yes", "--package", "@joinquest/mcp-integration", "joinquest-integration-mcp-cursor"],
      "env": {
        "JOINQUEST_API_KEY": "lq_dev_PASTE_HERE"
      }
    }
  }
}
```

### Claude Code (recommended — plugin)

Install the JoinQuest Claude Code plugin (skill + MCP in one step). From the dashboard → Claude Code tab, copy the install command.

Or run manually:

```bash
export JOINQUEST_API_KEY=lq_dev_PASTE_HERE
curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/playhub/main/scripts/install-joinquest-claude-plugin.sh | sh
```

Start a new Claude Code session in your game repo (or run `/reload-plugins`).

**Manual MCP only:**

```bash
claude mcp add --scope project --transport stdio \
  --env JOINQUEST_API_KEY=lq_dev_PASTE_HERE \
  joinquest-integration -- npx -y @joinquest/mcp-integration
```

### Claude Desktop

From your game repo:

```bash
JOINQUEST_API_KEY=lq_dev_PASTE_HERE curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/playhub/main/scripts/install-joinquest-dev.sh | sh -s -- --claude-desktop
```

That merges MCP into Claude Desktop's config (macOS / Windows / Linux). Fully quit Claude Desktop and reopen.

Or paste manually via **Settings → Developer → Edit Config** (same `mcpServers` JSON as other stdio clients, `args`: `["-y", "@joinquest/mcp-integration"]`).

### Gemini CLI

From your **game repo root**:

```bash
JOINQUEST_API_KEY=lq_dev_PASTE_HERE npx joinquest install gemini
```

That adds `.agents/skills/joinquest-integration/` and runs `gemini mcp add` (project scope) — or writes `.gemini/settings.json` if the CLI is unavailable.

**Manual MCP only:**

```bash
gemini mcp add -s project -t stdio \
  -e JOINQUEST_API_KEY=lq_dev_PASTE_HERE \
  joinquest-integration npx -y @joinquest/mcp-integration
```

Start a new Gemini CLI session in your game repo. Run `/mcp list` to confirm `joinquest-integration` is connected.

Browser Gemini / AI Studio does not support local stdio MCP — use the [manual browser registration](https://joinquest.cc/developers?path=manual) flow instead.

### GitHub Copilot, Roo Code, Windsurf, Cline

From your **game repo root** — dashboard tab for your editor, or:

```bash
JOINQUEST_API_KEY=lq_dev_PASTE_HERE npx joinquest install copilot
```

Platforms: `copilot`, `roo`, `windsurf`, `cline`. Each adds `.agents/skills/joinquest-integration/`, platform rules, and MCP config.

| Platform | MCP config | Notes |
|----------|------------|-------|
| **Copilot** | `.vscode/mcp.json` (`servers` key, not `mcpServers`) | MCP-first — Copilot may truncate long skills; call `joinquest_integration_get_agent_playbook` |
| **Roo** | `.roo/mcp.json` (committable) | Rules at `.roo/rules/joinquest-integration/` |
| **Windsurf** | `~/.codeium/windsurf/mcp_config.json` | Global only; export `JOINQUEST_API_KEY` before launch |
| **Cline** | Cline MCP settings (VS Code globalStorage) | Rules at `.cline/rules/joinquest-integration/` + `.clinerules` |
| **Gemini CLI** | `.gemini/settings.json` (project scope) | `gemini mcp add` or manual JSON; browser Gemini not supported |

Restart the editor / agent panel after install.

### ChatGPT (not supported)

JoinQuest MCP runs locally via `npx` (stdio). ChatGPT connectors need a hosted HTTPS MCP endpoint — not available from a game-repo install today.

**Manual browser flow:** [Register in the browser](https://joinquest.cc/developers?path=manual) — fill out the form on joinquest.cc, then use the game dashboard for checks and release.

3. **Restart the agent client fully** — Cursor: **Cmd+Q**, reopen, new Agent chat. VS Code / Windsurf: full quit or reload per editor.

4. **Verify** — ask the agent: “List my JoinQuest games using the MCP tools.”

5. **Start** — say: “I want to create a game on JoinQuest.” (Same prompt works if your game is already registered — the agent continues integration from there.)

## Agent skill + unified install (recommended for game repos)

From your **game repo root**. [Read the install script on GitHub](https://github.com/scruffyprodigy/playhub/blob/main/scripts/install-joinquest-dev.sh) before running.

It copies `.agents/skills/joinquest-integration/` from GitHub, merges MCP for your platform, and adds platform rules where helpful. MCP only calls joinquest.cc when tools run.

**Review first:**

```bash
curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/playhub/main/scripts/install-joinquest-dev.sh -o install-joinquest-dev.sh
less install-joinquest-dev.sh
JOINQUEST_API_KEY=lq_dev_... bash install-joinquest-dev.sh --copilot
```

**Quick install:**

```bash
JOINQUEST_API_KEY=lq_dev_... npx joinquest install roo
```

Platforms: `cursor`, `claude`, `claude-desktop`, `copilot`, `roo`, `windsurf`, `cline`, `gemini`, `skill`.

**Plugins (no game-repo changes):** Cursor and Claude Code — use `install-joinquest-cursor-plugin.sh` or `install-joinquest-claude-plugin.sh` instead.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Disconnected / no tools (Cursor) | Use `--package` + `joinquest-integration-mcp-cursor` bin (see config above) |
| Copilot ignores long playbook | MCP-first — call `joinquest_integration_get_agent_playbook` |
| Windsurf can't auth | Export `JOINQUEST_API_KEY` in shell before launching Windsurf |
| Auth errors | Regenerate API key on the dashboard |
| Invalid JSON | Validate the platform MCP config file |
| Still broken | Fully quit the client (**Cmd+Q** on Mac) |

Roadmap: [docs/developer-ai-setup-roadmap.md](../../docs/developer-ai-setup-roadmap.md)

## Available tools

See `playbook.md` § MCP tool quick reference, or MCP `joinquest_integration_get_agent_playbook`.
