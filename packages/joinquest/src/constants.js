export const MCP_PACKAGE = '@joinquest/mcp-integration'
export const MCP_SERVER_NAME = 'joinquest-integration'

export const PLATFORMS = [
  'cursor',
  'claude',
  'claude-desktop',
  'copilot',
  'roo',
  'windsurf',
  'cline',
  'gemini',
  'skill',
]

/** Platforms that support --plugin (global install, no game-repo files). */
export const PLUGIN_PLATFORMS = new Set(['cursor', 'claude'])
