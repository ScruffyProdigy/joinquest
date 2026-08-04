import { spawnSync as defaultSpawnSync } from 'node:child_process'
import { join } from 'node:path'
import { PLUGIN_PLATFORMS, MCP_PACKAGE, MCP_SERVER_NAME } from './constants.js'
import { buildStdioMcpServer, claudeMcpAddCommand, geminiMcpAddCommand, mergeMcpConfig } from './mcp-config.js'
import {
  installClineArtifacts,
  installCopilotArtifacts,
  installRooArtifacts,
  installWindsurfArtifacts,
} from './platforms.js'
import { installClaudePlugin, installCursorPlugin } from './plugin.js'
import { claudeDesktopConfigPath, clineMcpConfigPath, geminiProjectSettingsPath, windsurfMcpConfigPath } from './paths.js'
import { installAgentSkill } from './skill.js'

function requireApiKey(apiKey, platform, { plugin = false, skillOnly = false }) {
  if (skillOnly) return apiKey
  if (plugin && PLUGIN_PLATFORMS.has(platform)) return apiKey
  if (!apiKey) {
    throw new Error(
      'JOINQUEST_API_KEY is required. Generate one at https://joinquest.cc/developers → Connect an AI assistant.',
    )
  }
  return apiKey
}

function logAction(dryRun, message) {
  console.log(dryRun ? `[dry-run] ${message}` : message)
}

export function dryRunPlan(platform, options = {}) {
  const { plugin = false, apiKey = process.env.JOINQUEST_API_KEY || '' } = options
  const actions = []

  if (platform !== 'skill' && !(plugin && PLUGIN_PLATFORMS.has(platform))) {
    actions.push('Install .agents/skills/joinquest-integration/ in the current directory')
  }

  if (plugin && platform === 'cursor') {
    actions.push(`Install Cursor plugin → ~/.cursor/plugins/local/joinquest`)
    if (!apiKey) actions.push('(JOINQUEST_API_KEY required)')
    return actions
  }
  if (plugin && platform === 'claude') {
    actions.push(`Install Claude Code plugin → ~/.claude/skills/joinquest-integration`)
    if (!apiKey) actions.push('(JOINQUEST_API_KEY required)')
    return actions
  }

  switch (platform) {
    case 'skill':
      actions.push('Install .agents/skills/joinquest-integration/ only (no MCP config)')
      break
    case 'cursor':
      actions.push('Write MCP config → .cursor/mcp.json')
      break
    case 'claude':
      actions.push('Run claude mcp add for this project (or print manual command)')
      break
    case 'claude-desktop':
      actions.push('Write MCP config → Claude Desktop config file')
      break
    case 'copilot':
      actions.push('Install .github/skills/joinquest-integration/ + copilot-instructions.md')
      actions.push('Write MCP config → .vscode/mcp.json')
      break
    case 'roo':
      actions.push('Install .roo/rules/joinquest-integration/')
      actions.push('Write MCP config → .roo/mcp.json')
      break
    case 'windsurf':
      actions.push('Install .windsurf/rules/joinquest-integration/')
      actions.push('Write MCP config → ~/.codeium/windsurf/mcp_config.json')
      actions.push('Export JOINQUEST_API_KEY in your shell before starting Windsurf')
      break
    case 'cline':
      actions.push('Install .cline/rules/joinquest-integration/ + .clinerules')
      actions.push('Write MCP config → Cline globalStorage settings')
      break
    case 'gemini':
      actions.push('Run gemini mcp add for this project (or write .gemini/settings.json)')
      break
    default:
      actions.push(`Unknown platform: ${platform}`)
  }

  if (platform !== 'skill' && !apiKey) {
    actions.push('(JOINQUEST_API_KEY required for MCP setup)')
  }
  actions.push('At agent runtime: npx @joinquest/mcp-integration (not run during install)')
  return actions
}

export async function runInstall(platform, options = {}) {
  const {
    cwd = process.cwd(),
    dryRun = false,
    plugin = false,
    apiKey = process.env.JOINQUEST_API_KEY || '',
    spawnSync: spawn = defaultSpawnSync,
  } = options

  const normalized = platform.toLowerCase()

  if (dryRun) {
    console.log('JoinQuest install — dry run (no file writes).\n')
    for (const line of dryRunPlan(normalized, { plugin, apiKey })) {
      console.log(`  • ${line}`)
    }
    return { platform: normalized, dryRun: true }
  }

  if (plugin) {
    requireApiKey(apiKey, normalized, { plugin: true })
    if (normalized === 'cursor') {
      const dest = installCursorPlugin(apiKey, { dryRun })
      logAction(false, `Installed Cursor plugin to ${dest}`)
      console.log('Quit Cursor completely (Cmd+Q), reopen your game project, then start a fresh Agent chat.')
      return { platform: normalized, plugin: true }
    }
    if (normalized === 'claude') {
      const dest = installClaudePlugin(apiKey, { dryRun })
      logAction(false, `Installed Claude Code plugin to ${dest}`)
      console.log('Start a new Claude Code session in your game repo.')
      return { platform: normalized, plugin: true }
    }
    throw new Error('--plugin is only supported for cursor and claude.')
  }

  if (normalized === 'skill') {
    installAgentSkill(cwd, { dryRun })
    logAction(dryRun, `Install agent skill → ${join(cwd, '.agents/skills/joinquest-integration')}`)
    if (!dryRun) {
      console.log('\nNext: set JOINQUEST_API_KEY and run: npx joinquest install <platform>')
    }
    return { platform: normalized, skillOnly: true }
  }

  installAgentSkill(cwd, { dryRun })
  logAction(dryRun, `Install agent skill → ${join(cwd, '.agents/skills/joinquest-integration')}`)

  const key = requireApiKey(apiKey, normalized, { skillOnly: false })

  switch (normalized) {
    case 'cursor': {
      mergeMcpConfig(join(cwd, '.cursor/mcp.json'), 'mcpServers', buildStdioMcpServer({ apiKey: key, useCursorBin: true }))
      console.log('Fully quit Cursor (Cmd+Q), reopen this project, and start a new Agent chat.')
      break
    }
    case 'claude': {
      const cmd = claudeMcpAddCommand(key)
      const result = spawn('claude', ['mcp', 'add', '--scope', 'project', '--transport', 'stdio', '--env', `JOINQUEST_API_KEY=${key}`, 'joinquest-integration', '--', 'npx', '-y', '@joinquest/mcp-integration'], {
        cwd,
        stdio: 'inherit',
      })
      if (result.status !== 0) {
        console.error('claude CLI failed — run manually:\n')
        console.error(cmd)
      } else {
        console.log('Added joinquest-integration to .mcp.json (project scope).')
      }
      break
    }
    case 'claude-desktop': {
      mergeMcpConfig(claudeDesktopConfigPath(), 'mcpServers', buildStdioMcpServer({ apiKey: key }))
      console.log('Fully quit Claude Desktop, reopen it, and start a new conversation.')
      break
    }
    case 'copilot': {
      installCopilotArtifacts(cwd, { dryRun })
      mergeMcpConfig(join(cwd, '.vscode/mcp.json'), 'servers', buildStdioMcpServer({ apiKey: key }))
      console.log('Restart VS Code, enable Agent mode, and approve joinquest-integration MCP tools.')
      break
    }
    case 'roo': {
      installRooArtifacts(cwd, { dryRun })
      mergeMcpConfig(join(cwd, '.roo/mcp.json'), 'mcpServers', buildStdioMcpServer({ apiKey: key }))
      console.log('Reload VS Code or start a new Roo Code chat in this project.')
      break
    }
    case 'windsurf': {
      installWindsurfArtifacts(cwd, { dryRun })
      mergeMcpConfig(windsurfMcpConfigPath(), 'mcpServers', buildStdioMcpServer({ apiKey: key, windsurfEnv: true }))
      console.log(`Export JOINQUEST_API_KEY in your shell before starting Windsurf:`)
      console.log(`  export JOINQUEST_API_KEY=${key}`)
      console.log('Fully quit Windsurf (Cmd+Q), reopen, and start a fresh Cascade chat.')
      break
    }
    case 'cline': {
      installClineArtifacts(cwd, { dryRun })
      mergeMcpConfig(clineMcpConfigPath(), 'mcpServers', buildStdioMcpServer({ apiKey: key }))
      console.log('Reload the Cline panel in VS Code and approve joinquest-integration when prompted.')
      break
    }
    case 'gemini': {
      const settingsPath = geminiProjectSettingsPath(cwd)
      const result = spawn(
        'gemini',
        [
          'mcp',
          'add',
          '-s',
          'project',
          '-t',
          'stdio',
          '-e',
          `JOINQUEST_API_KEY=${key}`,
          MCP_SERVER_NAME,
          'npx',
          '-y',
          MCP_PACKAGE,
        ],
        { cwd, stdio: 'inherit' },
      )
      if (result.status !== 0) {
        mergeMcpConfig(settingsPath, 'mcpServers', buildStdioMcpServer({ apiKey: key }))
        console.log(`Wrote MCP config → ${settingsPath}`)
        if (result.error?.code === 'ENOENT') {
          console.error('gemini CLI not found. Install: https://github.com/google-gemini/gemini-cli')
        }
        console.error('gemini mcp add failed — run manually:\n')
        console.error(geminiMcpAddCommand(key))
      } else {
        console.log('Added joinquest-integration to .gemini/settings.json (project scope).')
      }
      console.log('Start a new Gemini CLI session in this project directory.')
      break
    }
    default:
      throw new Error(`Unknown platform "${platform}". Run: npx joinquest install --help`)
  }

  console.log('\nVerify: ask your agent — "List my JoinQuest games using the MCP tools."')
  console.log('Then start: "I want to create a game on JoinQuest."')
  return { platform: normalized }
}
