# Developer AI setup roadmap

Track simplifying JoinQuest integration for game developers using Cursor, Claude Code, and other agents. **Skill + MCP** are the two layers; the goal is one mental model and minimal steps.

## Target experience

| Audience | Done when |
|----------|-----------|
| **Cursor** | Dashboard → copy config → Cmd+Q → Agent chat → verify MCP → “I want to create a game on JoinQuest” |
| **Claude Code** | Dashboard → copy `claude mcp add …` → new session → approve MCP → same prompt |
| **Copilot / Roo / Windsurf / Cline / Gemini CLI** | Dashboard → copy `npx joinquest install <platform>` from game repo |
| **ChatGPT** | Not supported for MCP — use [Register in the browser](https://joinquest.cc/developers?path=manual) in ChatGPT’s browser |
| **Everyone** | Agent skill installed in game repo (`.agents/skills/` + platform rules) |

Production only for game devs: `JOINQUEST_API_KEY` in MCP env (no URL). Lobby contributors: see [development.md](./development.md) for local override.

---

## Tier 1 — Highest impact

| # | Item | Status | Notes |
|---|------|--------|-------|
| 1 | **Publish `@joinquest/mcp-integration` to npm** | Republish 0.1.1 | 0.1.0 missing default bin; Cursor wrapper path fix in 0.1.1 |
| 2 | **Dashboard copy-ready commands** | Done | One-click Copy Cursor JSON + Copy Claude command after key gen |
| 3 | **Unified setup script** (skill + MCP hints) | Done | `install-joinquest-dev.sh` — all platform flags |
| 4 | **Skill install automation** | Done | Bundled in `install-joinquest-dev.sh` |
| 5 | **Deploy doc/wizard/MCP fixes to joinquest.cc** | Done | Live on joinquest.cc (multi-platform wizard + ChatGPT manual path) |

## Tier 2 — Remove secrets & friction

| # | Item | Status | Notes |
|---|------|--------|-------|
| 6 | **`joinquest login` / device auth for MCP** | Pending | No API key in `.cursor/mcp.json` |
| 7 | **One curl URL for all platforms** | Done | `install-joinquest-dev.sh` + dashboard tabs; plugin scripts for Cursor/Claude |
| 8 | **Cursor MCP reliability** | Ongoing | `joinquest-integration-mcp-cursor` wrapper; consider HTTP MCP later |

## Tier 3 — Polish & reach

| # | Item | Status | Notes |
|---|------|--------|-------|
| 9 | **npx-first docs everywhere** | In progress | After npm publish, install script = fallback only |
| 10 | **Claude Desktop / Copilot / Roo / Windsurf / Cline / Gemini CLI** | Done | `npx joinquest install <platform>` + dashboard tabs |
| 11 | **ChatGPT connector** | Pending | Requires hosted HTTPS MCP + OAuth; manual browser flow documented |
| 12 | **Dashboard API key management UX** | Pending | Clear list/revoke/regenerate for MCP |

## Explicitly not now

- Per-IDE extensions
- Localhost in game-developer funnel
- Assuming Cursor “Tools & MCP” settings UI exists

---

## MCP config reference (npx, post-publish)

**Cursor** — needs cursor bin via npx (Electron env workaround):

```json
{
  "mcpServers": {
    "joinquest-integration": {
      "type": "stdio",
      "command": "npx",
      "args": ["--yes", "--package", "@joinquest/mcp-integration", "joinquest-integration-mcp-cursor"],
      "env": { "JOINQUEST_API_KEY": "lq_dev_..." }
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

**GitHub Copilot** — `.vscode/mcp.json` uses `servers` (not `mcpServers`):

```json
{
  "servers": {
    "joinquest-integration": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@joinquest/mcp-integration"],
      "env": { "JOINQUEST_API_KEY": "lq_dev_..." }
    }
  }
}
```

**Other stdio clients** (Roo, Windsurf, Cline, Claude Desktop):

```json
"command": "npx",
"args": ["-y", "@joinquest/mcp-integration"]
```

Game-repo install: `install-joinquest-dev.sh --<platform>` from the dashboard.

---

## Changelog

- 2026-06-13 — Roadmap created; MCP package npx-first configs landed; publish pending npm auth.
- 2026-06-13 — `@joinquest/mcp-integration@0.1.0` published to npm.
- 2026-06-15 — Multi-platform install (`--copilot`, `--roo`, `--windsurf`, `--cline`) + dashboard tabs; ChatGPT manual path at `?path=manual`.
- 2026-06-15 — Docs cleanup: expanded `mcp-setup.md`, playbook ChatGPT note, Cline rules parity.
