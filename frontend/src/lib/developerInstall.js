/**
 * Copy-paste install snippets for the developer dashboard: the `npx joinquest
 * install` commands, the per-client MCP server config, and the URLs the wizard
 * links to. Nothing here talks to the GraphQL API — it only renders strings a
 * developer pastes into their own editor or shell.
 */

function joinquestMcpEnv({ apiKey }) {
  return {
    JOINQUEST_API_KEY: apiKey || '<generate-on-dashboard>',
  }
}

const MCP_NPX_PACKAGE = '@joinquest/mcp-integration'
export const JOINQUEST_CLI_PACKAGE = 'joinquest'
export const INSTALL_DEV_SCRIPT_BASE =
  'https://raw.githubusercontent.com/scruffyprodigy/joinquest/main/scripts'
export const INSTALL_DEV_SCRIPT_URL = `${INSTALL_DEV_SCRIPT_BASE}/install-joinquest-dev.sh`
export const INSTALL_DEV_SCRIPT_GITHUB =
  'https://github.com/scruffyprodigy/joinquest/blob/main/scripts/install-joinquest-dev.sh'
export const INSTALL_SETUP_MANIFEST_GITHUB =
  'https://github.com/scruffyprodigy/joinquest/blob/main/scripts/joinquest-setup/README.md'
export const JOINQUEST_CLI_GITHUB =
  'https://github.com/scruffyprodigy/joinquest/tree/main/packages/joinquest'
/** @deprecated Shell libs — use npx joinquest install instead. */
export const INSTALL_DEV_LIB_FILES = [
  'joinquest-skill.sh',
  'joinquest-mcp-env.sh',
  'joinquest-mcp-config.sh',
  'joinquest-platform.sh',
]
export const INSTALL_CURSOR_PLUGIN_SCRIPT_URL =
  'https://raw.githubusercontent.com/scruffyprodigy/joinquest/main/scripts/install-joinquest-cursor-plugin.sh'
export const INSTALL_CURSOR_PLUGIN_SCRIPT_GITHUB =
  'https://github.com/scruffyprodigy/joinquest/blob/main/scripts/install-joinquest-cursor-plugin.sh'
export const INSTALL_CLAUDE_PLUGIN_SCRIPT_URL =
  'https://raw.githubusercontent.com/scruffyprodigy/joinquest/main/scripts/install-joinquest-claude-plugin.sh'
export const INSTALL_CLAUDE_PLUGIN_SCRIPT_GITHUB =
  'https://github.com/scruffyprodigy/joinquest/blob/main/scripts/install-joinquest-claude-plugin.sh'
export const CURSOR_PLUGIN_GITHUB =
  'https://github.com/scruffyprodigy/joinquest/tree/main/plugins/joinquest'

function joinquestInstallPlatform(client) {
  switch (client) {
    case 'claude':
    case 'claude-code':
      return 'claude'
    case 'claude-desktop':
      return 'claude-desktop'
    case 'copilot':
      return 'copilot'
    case 'roo':
      return 'roo'
    case 'windsurf':
      return 'windsurf'
    case 'cline':
      return 'cline'
    case 'gemini':
      return 'gemini'
    case 'skill-only':
      return 'skill'
    default:
      return 'cursor'
  }
}

function joinquestInstallEnvPrefix(apiKey) {
  const key = apiKey || 'lq_dev_PASTE_YOUR_KEY'
  return `JOINQUEST_API_KEY=${key}`
}

export function buildJoinquestInstallCommand({ apiKey, client = 'cursor', plugin = false }) {
  const platform = joinquestInstallPlatform(client)
  const flags = plugin && (platform === 'cursor' || platform === 'claude') ? ' --plugin' : ''
  return `${joinquestInstallEnvPrefix(apiKey)}
npx -y ${JOINQUEST_CLI_PACKAGE} install ${platform}${flags}`
}

export function buildJoinquestCreateCommand({ apiKey, client = 'cursor' }) {
  const platform = joinquestInstallPlatform(client)
  return `${joinquestInstallEnvPrefix(apiKey)}
npm create joinquest@latest -- --${platform}`
}

export function buildInstallCursorPluginCommand({ apiKey }) {
  return buildJoinquestInstallCommand({ apiKey, client: 'cursor', plugin: true })
}

export function buildInstallClaudePluginCommand({ apiKey }) {
  return buildJoinquestInstallCommand({ apiKey, client: 'claude', plugin: true })
}

function joinquestStdioMcpServer({ apiKey, clientId }) {
  const args =
    clientId === 'cursor'
      ? ['--yes', '--package', MCP_NPX_PACKAGE, 'joinquest-integration-mcp-cursor']
      : ['-y', MCP_NPX_PACKAGE]
  const env =
    clientId === 'windsurf'
      ? { JOINQUEST_API_KEY: '${env:JOINQUEST_API_KEY}' }
      : joinquestMcpEnv({ apiKey })
  return {
    type: 'stdio',
    command: 'npx',
    args,
    env,
  }
}

export function buildInstallDevCommand({ apiKey, client = 'cursor' }) {
  return buildJoinquestInstallCommand({ apiKey, client })
}

export function buildInstallDevDryRunCommand({ client = 'cursor' }) {
  const platform = joinquestInstallPlatform(client)
  return `npx -y ${JOINQUEST_CLI_PACKAGE} install ${platform} --dry-run`
}

export function buildInstallDevInspectCommand({ apiKey, client = 'cursor' }) {
  const dryRun = buildInstallDevDryRunCommand({ client })
  const install = buildJoinquestInstallCommand({ apiKey, client })
  return `# Preview planned actions (no writes):
${dryRun}

# When ready:
${install}

# Package source: ${JOINQUEST_CLI_GITHUB}`
}

export function buildClaudeMcpAddCommand({ apiKey }) {
  const key = apiKey || 'lq_dev_PASTE_HERE'
  return `claude mcp add --scope project --transport stdio \\
  --env JOINQUEST_API_KEY=${key} \\
  joinquest-integration -- npx -y ${MCP_NPX_PACKAGE}`
}

export function buildGeminiMcpAddCommand({ apiKey }) {
  const key = apiKey || 'lq_dev_PASTE_HERE'
  return `gemini mcp add -s project -t stdio \\
  -e JOINQUEST_API_KEY=${key} \\
  joinquest-integration npx -y ${MCP_NPX_PACKAGE}`
}

export function buildMcpServerConfig({ apiKey, clientId = 'cursor' }) {
  const server = joinquestStdioMcpServer({
    apiKey: apiKey ?? '<paste-api-key-here>',
    clientId,
  })
  if (clientId === 'copilot') {
    return {
      servers: {
        'joinquest-integration': server,
      },
    }
  }
  return {
    mcpServers: {
      'joinquest-integration': server,
    },
  }
}

/**
 * Claude Code wants the transport spelled out on the server entry. Applied on
 * top of buildMcpServerConfig rather than inside it, so the other clients keep
 * the exact JSON they already copy.
 */
export function buildClaudeCodeConfig(base) {
  const server = base.mcpServers['joinquest-integration']
  return {
    mcpServers: {
      'joinquest-integration': {
        type: 'stdio',
        ...server,
      },
    },
  }
}
