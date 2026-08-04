import assert from 'node:assert/strict'
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

export const TEST_API_KEY = 'lq_dev_test'

export function readJson(filePath) {
  return JSON.parse(readFileSync(filePath, 'utf8'))
}

export function assertSkillInstalled(cwd) {
  assert.ok(
    existsSync(join(cwd, '.agents/skills/joinquest-integration/SKILL.md')),
    'expected agent skill at .agents/skills/joinquest-integration/SKILL.md',
  )
}

export function assertMcpServer(config, { rootKey = 'mcpServers', apiKey = TEST_API_KEY, windsurf = false } = {}) {
  const server = config[rootKey]?.['joinquest-integration']
  assert.ok(server, `expected joinquest-integration under ${rootKey}`)
  assert.equal(server.command, 'npx')
  assert.ok(Array.isArray(server.args) && server.args.length > 0)
  if (windsurf) {
    assert.equal(server.env.JOINQUEST_API_KEY, '${env:JOINQUEST_API_KEY}')
  } else {
    assert.equal(server.env.JOINQUEST_API_KEY, apiKey)
  }
  return server
}

export async function withTempDir(prefix, fn) {
  const dir = mkdtempSync(join(tmpdir(), prefix))
  try {
    return await fn(dir)
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
}

export async function withTestHome(fn) {
  return withTempDir('joinquest-home-', async (home) => {
    const previous = process.env.JOINQUEST_INSTALL_TEST_HOME
    process.env.JOINQUEST_INSTALL_TEST_HOME = home
    try {
      return await fn(home)
    } finally {
      if (previous === undefined) {
        delete process.env.JOINQUEST_INSTALL_TEST_HOME
      } else {
        process.env.JOINQUEST_INSTALL_TEST_HOME = previous
      }
    }
  })
}

export async function withProjectDir(fn) {
  return withTempDir('joinquest-project-', fn)
}
