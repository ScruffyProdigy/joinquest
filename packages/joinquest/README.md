# joinquest

Install the JoinQuest agent skill and MCP config for your AI editor — **one npm package**, no `curl | sh`.

## Quick start

```bash
JOINQUEST_API_KEY=lq_dev_... npx joinquest install cursor
```

Or:

```bash
npm create joinquest@latest -- --cursor
```

Get an API key: [joinquest.cc/developers](https://joinquest.cc/developers) → **Connect an AI assistant**.

## Platforms

```bash
npx joinquest install cursor
npx joinquest install claude
npx joinquest install claude-desktop
npx joinquest install copilot
npx joinquest install roo
npx joinquest install windsurf
npx joinquest install cline
npx joinquest install gemini
npx joinquest install skill          # skill only, no MCP
```

**Global plugin** (Cursor / Claude Code — no game-repo changes):

```bash
npx joinquest install cursor --plugin
npx joinquest install claude --plugin
```

**Preview without writing files:**

```bash
npx joinquest install copilot --dry-run
```

## What this installs

| Component | Source |
|-----------|--------|
| Agent skill | Bundled in this package → `.agents/skills/joinquest-integration/` |
| MCP config | Points at `npx @joinquest/mcp-integration` (already on npm) |
| Editor rules | Copilot / Roo / Cline / Windsurf paths as needed |

Nothing else is downloaded at install time.

## MCP runtime

After setup, your editor runs MCP via:

```bash
npx @joinquest/mcp-integration
```

(Cursor uses the `joinquest-integration-mcp-cursor` bin — configured automatically.)

## Options

| Flag | Description |
|------|-------------|
| `--dry-run` | Print planned actions only |
| `--plugin` | Install to `~/.cursor` or `~/.claude` (cursor/claude only) |
| `--api-key <key>` | API key (or `JOINQUEST_API_KEY` env) |

## Publishing (maintainers)

```bash
./scripts/publish-joinquest-cli.sh
```

Publishes `joinquest` then `create-joinquest` to npm.

