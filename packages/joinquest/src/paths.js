import { homedir } from 'node:os'
import { join } from 'node:path'

/** Override with JOINQUEST_INSTALL_TEST_HOME in tests (isolated fake $HOME). */
export function userHome() {
  return process.env.JOINQUEST_INSTALL_TEST_HOME || homedir()
}

export function claudeDesktopConfigPath() {
  switch (process.platform) {
    case 'darwin':
      return join(userHome(), 'Library/Application Support/Claude/claude_desktop_config.json')
    case 'win32':
      return join(process.env.APPDATA || join(userHome(), 'AppData/Roaming'), 'Claude/claude_desktop_config.json')
    default:
      return join(process.env.XDG_CONFIG_HOME || join(userHome(), '.config'), 'Claude/claude_desktop_config.json')
  }
}

export function windsurfMcpConfigPath() {
  return join(userHome(), '.codeium/windsurf/mcp_config.json')
}

export function clineMcpConfigPath() {
  switch (process.platform) {
    case 'darwin':
      return join(
        userHome(),
        'Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json',
      )
    case 'win32':
      return join(
        process.env.APPDATA || join(userHome(), 'AppData/Roaming'),
        'Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json',
      )
    default:
      return join(
        process.env.XDG_CONFIG_HOME || join(userHome(), '.config'),
        'Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json',
      )
  }
}

export function cursorPluginDir() {
  return join(userHome(), '.cursor/plugins/local/joinquest')
}

export function claudePluginDir() {
  return join(userHome(), '.claude/skills/joinquest-integration')
}

export function geminiProjectSettingsPath(cwd) {
  return join(cwd, '.gemini/settings.json')
}
