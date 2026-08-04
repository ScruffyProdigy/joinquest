import { mkdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs'
import { dirname } from 'node:path'
import { MCP_PACKAGE, MCP_SERVER_NAME } from './constants.js'

export function buildStdioMcpServer({ apiKey, useCursorBin = false, windsurfEnv = false }) {
  const args = useCursorBin
    ? ['--yes', '--package', MCP_PACKAGE, 'joinquest-integration-mcp-cursor']
    : ['-y', MCP_PACKAGE]

  const env = windsurfEnv
    ? { JOINQUEST_API_KEY: '${env:JOINQUEST_API_KEY}' }
    : { JOINQUEST_API_KEY: apiKey }

  return {
    type: 'stdio',
    command: 'npx',
    args,
    env,
  }
}

export function mergeMcpConfig(configPath, rootKey, server, { dryRun = false } = {}) {
  let config = {}
  if (existsSync(configPath)) {
    config = JSON.parse(readFileSync(configPath, 'utf8'))
  }
  if (!config[rootKey] || typeof config[rootKey] !== 'object') {
    config[rootKey] = {}
  }
  config[rootKey][MCP_SERVER_NAME] = server

  if (dryRun) {
    return { configPath, rootKey, wrote: false }
  }

  mkdirSync(dirname(configPath), { recursive: true })
  writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`)
  return { configPath, rootKey, wrote: true }
}

export function claudeMcpAddCommand(apiKey) {
  return `claude mcp add --scope project --transport stdio \\
  --env JOINQUEST_API_KEY=${apiKey} \\
  ${MCP_SERVER_NAME} -- npx -y ${MCP_PACKAGE}`
}

export function geminiMcpAddCommand(apiKey) {
  return `gemini mcp add -s project -t stdio \\
  -e JOINQUEST_API_KEY=${apiKey} \\
  ${MCP_SERVER_NAME} npx -y ${MCP_PACKAGE}`
}
