import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync, existsSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { buildStdioMcpServer, mergeMcpConfig } from '../src/mcp-config.js'
import { parseInstallArgv, runCli } from '../src/cli.js'
import { dryRunPlan } from '../src/install.js'

test('buildStdioMcpServer uses cursor bin when requested', () => {
  const server = buildStdioMcpServer({ apiKey: 'lq_dev_test', useCursorBin: true })
  assert.equal(server.command, 'npx')
  assert.ok(server.args.includes('joinquest-integration-mcp-cursor'))
  assert.equal(server.env.JOINQUEST_API_KEY, 'lq_dev_test')
})

test('mergeMcpConfig merges into existing file', () => {
  const dir = mkdtempSync(join(tmpdir(), 'joinquest-mcp-'))
  const path = join(dir, '.cursor/mcp.json')
  const { wrote } = mergeMcpConfig(
    path,
    'mcpServers',
    buildStdioMcpServer({ apiKey: 'k', useCursorBin: true }),
  )
  assert.equal(wrote, true)
  const config = JSON.parse(readFileSync(path, 'utf8'))
  assert.ok(config.mcpServers['joinquest-integration'])
})

test('parseInstallArgv accepts legacy --cursor flag', () => {
  const parsed = parseInstallArgv(['--cursor', '--dry-run'])
  assert.deepEqual(parsed.positional, ['cursor'])
  assert.equal(parsed.dryRun, true)
})

test('dryRunPlan lists skill install for copilot', () => {
  const actions = dryRunPlan('copilot', { apiKey: 'k' })
  assert.ok(actions.some((a) => a.includes('.github/skills')))
  assert.ok(actions.some((a) => a.includes('.vscode/mcp.json')))
})

test('dryRunPlan lists gemini mcp add', () => {
  const actions = dryRunPlan('gemini', { apiKey: 'k' })
  assert.ok(actions.some((a) => a.includes('.agents/skills')))
  assert.ok(actions.some((a) => a.includes('gemini mcp add')))
})

test('runCli install skill dry-run exits 0', async () => {
  const code = await runCli(['install', 'skill', '--dry-run'])
  assert.equal(code, 0)
})

test('skill install writes SKILL.md', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'joinquest-install-'))
  const cwd = process.cwd()
  process.chdir(dir)
  try {
    const code = await runCli(['install', 'skill'])
    assert.equal(code, 0)
    assert.ok(existsSync(join(dir, '.agents/skills/joinquest-integration/SKILL.md')))
  } finally {
    process.chdir(cwd)
  }
})
