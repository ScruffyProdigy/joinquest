import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { test } from 'node:test'
import { PLATFORMS } from '../src/constants.js'
import { dryRunPlan } from '../src/install.js'
import {
  claudeDesktopConfigPath,
  clineMcpConfigPath,
  windsurfMcpConfigPath,
} from '../src/paths.js'
import {
  TEST_API_KEY,
  assertMcpServer,
  assertSkillInstalled,
  readJson,
  withProjectDir,
  withTestHome,
} from './helpers.js'

async function loadInstall() {
  return import('../src/install.js')
}

test('dryRunPlan covers every platform without unknown actions', () => {
  for (const platform of PLATFORMS) {
    const actions = dryRunPlan(platform, { apiKey: TEST_API_KEY })
    assert.ok(actions.length > 0, `dryRunPlan returned no actions for ${platform}`)
    assert.ok(
      !actions.some((line) => line.startsWith('Unknown platform')),
      `dryRunPlan has unknown platform for ${platform}`,
    )
  }
})

test('skill install writes only the agent skill', async () => {
  await withProjectDir(async (cwd) => {
    const { runInstall } = await loadInstall()
    await runInstall('skill', { cwd })
    assertSkillInstalled(cwd)
    assert.equal(existsSync(join(cwd, '.cursor/mcp.json')), false)
  })
})

test('cursor install writes skill and mcp.json with cursor bin', async () => {
  await withProjectDir(async (cwd) => {
    const { runInstall } = await loadInstall()
    await runInstall('cursor', { cwd, apiKey: TEST_API_KEY })
    assertSkillInstalled(cwd)
    const config = readJson(join(cwd, '.cursor/mcp.json'))
    const server = assertMcpServer(config)
    assert.ok(server.args.includes('joinquest-integration-mcp-cursor'))
  })
})

test('copilot install writes skill, rules, and vscode mcp config', async () => {
  await withProjectDir(async (cwd) => {
    const { runInstall } = await loadInstall()
    await runInstall('copilot', { cwd, apiKey: TEST_API_KEY })
    assertSkillInstalled(cwd)
    assert.ok(existsSync(join(cwd, '.github/skills/joinquest-integration/SKILL.md')))
    assert.ok(existsSync(join(cwd, '.github/copilot-instructions.md')))
    const config = readJson(join(cwd, '.vscode/mcp.json'))
    assertMcpServer(config, { rootKey: 'servers' })
  })
})

test('roo install writes skill, rules, and roo mcp config', async () => {
  await withProjectDir(async (cwd) => {
    const { runInstall } = await loadInstall()
    await runInstall('roo', { cwd, apiKey: TEST_API_KEY })
    assertSkillInstalled(cwd)
    assert.ok(existsSync(join(cwd, '.roo/rules/joinquest-integration/SKILL.md')))
    assertMcpServer(readJson(join(cwd, '.roo/mcp.json')))
  })
})

test('windsurf install writes skill, rules, and global mcp config', async () => {
  await withTestHome(async () => {
    await withProjectDir(async (cwd) => {
      const { runInstall } = await loadInstall()
      await runInstall('windsurf', { cwd, apiKey: TEST_API_KEY })
      assertSkillInstalled(cwd)
      assert.ok(existsSync(join(cwd, '.windsurf/rules/joinquest-integration/SKILL.md')))
      assertMcpServer(readJson(windsurfMcpConfigPath()), { windsurf: true })
    })
  })
})

test('cline install writes skill, rules, clinerules, and global mcp config', async () => {
  await withTestHome(async () => {
    await withProjectDir(async (cwd) => {
      const { runInstall } = await loadInstall()
      await runInstall('cline', { cwd, apiKey: TEST_API_KEY })
      assertSkillInstalled(cwd)
      assert.ok(existsSync(join(cwd, '.cline/rules/joinquest-integration/SKILL.md')))
      assert.ok(existsSync(join(cwd, '.clinerules')))
      assertMcpServer(readJson(clineMcpConfigPath()))
    })
  })
})

test('claude-desktop install writes skill and desktop mcp config', async () => {
  await withTestHome(async () => {
    await withProjectDir(async (cwd) => {
      const { runInstall } = await loadInstall()
      await runInstall('claude-desktop', { cwd, apiKey: TEST_API_KEY })
      assertSkillInstalled(cwd)
      assertMcpServer(readJson(claudeDesktopConfigPath()))
    })
  })
})

test('gemini install writes skill and project settings when CLI is unavailable', async () => {
  await withProjectDir(async (cwd) => {
    const { runInstall } = await loadInstall()
    await runInstall('gemini', { cwd, apiKey: TEST_API_KEY })
    assertSkillInstalled(cwd)
    assertMcpServer(readJson(join(cwd, '.gemini/settings.json')))
  })
})

test('claude install invokes claude mcp add', async () => {
  const spawnCalls = []
  await withProjectDir(async (cwd) => {
    const { runInstall } = await loadInstall()
    await runInstall('claude', {
      cwd,
      apiKey: TEST_API_KEY,
      spawnSync: (...args) => {
        spawnCalls.push(args)
        return { status: 0 }
      },
    })
    assertSkillInstalled(cwd)
    assert.equal(spawnCalls.length, 1)
    assert.equal(spawnCalls[0][0], 'claude')
    assert.ok(spawnCalls[0][1].includes('mcp'))
    assert.ok(spawnCalls[0][1].includes('joinquest-integration'))
  })
})

test('cursor plugin install writes plugin under test home', async () => {
  await withTestHome(async (home) => {
    const { runInstall } = await loadInstall()
    await runInstall('cursor', { plugin: true, apiKey: TEST_API_KEY })
    assert.ok(existsSync(join(home, '.cursor/plugins/local/joinquest/.cursor-plugin/plugin.json')))
  })
})
