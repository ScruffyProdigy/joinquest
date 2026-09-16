import { describe, expect, it } from 'vitest'

describe('buildMcpServerConfig', () => {
  it('uses mcpServers for Cursor', async () => {
    const { buildMcpServerConfig } = await import('./developerInstall')
    const config = buildMcpServerConfig({ apiKey: 'lq_dev_test', clientId: 'cursor' })
    expect(config.mcpServers['joinquest-integration'].args).toContain('joinquest-integration-mcp-cursor')
  })

  it('uses servers for Copilot', async () => {
    const { buildMcpServerConfig } = await import('./developerInstall')
    const config = buildMcpServerConfig({ apiKey: 'lq_dev_test', clientId: 'copilot' })
    expect(config.servers['joinquest-integration'].env.JOINQUEST_API_KEY).toBe('lq_dev_test')
    expect(config.mcpServers).toBeUndefined()
  })

  it('uses env interpolation for Windsurf', async () => {
    const { buildMcpServerConfig } = await import('./developerInstall')
    const config = buildMcpServerConfig({ apiKey: 'lq_dev_test', clientId: 'windsurf' })
    expect(config.mcpServers['joinquest-integration'].env.JOINQUEST_API_KEY).toBe('${env:JOINQUEST_API_KEY}')
  })
})

describe('buildInstallDevCommand', () => {
  it('maps platform flags', async () => {
    const { buildInstallDevCommand } = await import('./developerInstall')
    expect(buildInstallDevCommand({ apiKey: 'k', client: 'copilot' })).toContain('install copilot')
    expect(buildInstallDevCommand({ apiKey: 'k', client: 'roo' })).toContain('install roo')
    expect(buildInstallDevCommand({ apiKey: 'k', client: 'gemini' })).toContain('install gemini')
  })
})

describe('buildGeminiMcpAddCommand', () => {
  it('uses gemini mcp add with project scope', async () => {
    const { buildGeminiMcpAddCommand } = await import('./developerInstall')
    const cmd = buildGeminiMcpAddCommand({ apiKey: 'lq_dev_test' })
    expect(cmd).toContain('gemini mcp add -s project -t stdio')
    expect(cmd).toContain('JOINQUEST_API_KEY=lq_dev_test')
    expect(cmd).toContain('joinquest-integration')
  })
})

describe('buildInstallDevInspectCommand', () => {
  it('uses npx joinquest dry-run and install', async () => {
    const { buildInstallDevInspectCommand, JOINQUEST_CLI_PACKAGE } = await import('./developerInstall')
    const cmd = buildInstallDevInspectCommand({ apiKey: 'lq_dev_test', client: 'copilot' })
    expect(cmd).toContain(`npx -y ${JOINQUEST_CLI_PACKAGE} install copilot --dry-run`)
    expect(cmd).toContain('JOINQUEST_API_KEY=lq_dev_test')
    expect(cmd).toContain('install copilot')
  })
})

describe('buildInstallDevDryRunCommand', () => {
  it('uses npx joinquest install --dry-run', async () => {
    const { buildInstallDevDryRunCommand, JOINQUEST_CLI_PACKAGE } = await import('./developerInstall')
    expect(buildInstallDevDryRunCommand({ client: 'cursor' })).toBe(
      `npx -y ${JOINQUEST_CLI_PACKAGE} install cursor --dry-run`,
    )
  })
})

describe('buildClaudeCodeConfig', () => {
  it('spells out the stdio transport on top of the base config', async () => {
    const { buildClaudeCodeConfig, buildMcpServerConfig } = await import('./developerInstall')
    const base = buildMcpServerConfig({ apiKey: 'lq_dev_test', clientId: 'claude-code' })
    const config = buildClaudeCodeConfig(base)
    const server = config.mcpServers['joinquest-integration']
    expect(server.type).toBe('stdio')
    expect(server.command).toBe('npx')
    expect(server.env.JOINQUEST_API_KEY).toBe('lq_dev_test')
  })
})
