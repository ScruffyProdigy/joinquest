# JoinQuest integration plugin

Agent skill + MCP for integrating multiplayer games with [JoinQuest](https://joinquest.cc).

Works as a **Cursor plugin** (`.cursor-plugin/`, `mcp.json`) and a **Claude Code plugin** (`.claude-plugin/`, `.mcp.json`). Both share the same `skills/` directory.

**Also supported** via `install-joinquest-dev.sh`: GitHub Copilot, Roo Code, Windsurf, and Cline. ChatGPT is not supported yet (requires hosted MCP) — use [Register in the browser](https://joinquest.cc/developers?path=manual).

## Install

Generate an API key at [joinquest.cc/developers](https://joinquest.cc/developers) → **Connect an AI assistant**, then copy the install command for your editor.

### Cursor

```bash
export JOINQUEST_API_KEY=lq_dev_PASTE_YOUR_KEY
curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/playhub/main/scripts/install-joinquest-cursor-plugin.sh | sh
```

Quit Cursor completely (Cmd+Q), reopen your game project, and start a fresh Agent chat.

### Claude Code

```bash
export JOINQUEST_API_KEY=lq_dev_PASTE_YOUR_KEY
curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/playhub/main/scripts/install-joinquest-claude-plugin.sh | sh
```

Start a new Claude Code session in your game repo (or run `/reload-plugins`).

### GitHub Copilot / Roo Code / Windsurf / Cline

From the [developer dashboard](https://joinquest.cc/developers) → **Connect an AI assistant**, pick your editor, copy the install command.

Or from your game repo:

```bash
export JOINQUEST_API_KEY=lq_dev_PASTE_YOUR_KEY
curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/playhub/main/scripts/install-joinquest-dev.sh | sh -s -- --copilot   # or --roo, --windsurf, --cline
```

## What's included

| Component | Purpose |
|-----------|---------|
| `skills/joinquest-integration/` | Agent skill — phased playbook for discovery, API, checks, release |
| `skills/multiplayer-game-design/` | Server authority, disconnect/reconnect, turn structure, competitive balance, hidden information |
| `skills/game-architecture/` | Code structure — EventBus, centralized state, restart-safety, client-side prediction |
| `skills/phaser/` | Phaser 3 engine mechanics for 2D games |
| `skills/threejs-game/` | Three.js engine mechanics for 3D games |
| `mcp.json` / `.mcp.json` | MCP server via `npx @joinquest/mcp-integration` |

`joinquest-integration` gets you through the handshake. The other four are for
building the game itself, and they're deliberately generic — no JoinQuest
specifics — so they apply to any multiplayer browser game. Start with
`multiplayer-game-design`: every JoinQuest game has more than one real player in
it, and that's where the decisions that are expensive to reverse live.

## Local development

**Cursor:**

```bash
mkdir -p ~/.cursor/plugins/local
ln -sf "$(pwd)" ~/.cursor/plugins/local/joinquest
```

**Claude Code:**

```bash
mkdir -p ~/.claude/skills
ln -sf "$(pwd)" ~/.claude/skills/joinquest-integration
```

Reload the editor, then verify the skill and MCP server appear.

## Publish

- Cursor: [Cursor Marketplace](https://cursor.com/marketplace/publish)
- Claude Code: marketplace or team distribution via `.claude-plugin/marketplace.json`

Plugin source lives at `plugins/joinquest/` in the [playhub](https://github.com/scruffyprodigy/playhub) repository.
