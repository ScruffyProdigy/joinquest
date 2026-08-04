#!/usr/bin/env node
/**
 * Ensures joinquest CLI platforms, dashboard tabs, bundled assets, and install commands stay aligned.
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { PLATFORMS } from '../packages/joinquest/src/constants.js'
import { dryRunPlan } from '../packages/joinquest/src/install.js'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

function fail(message) {
  console.error(message)
  process.exit(1)
}

function parseDashboardInstallClients(source) {
  const clients = []
  for (const match of source.matchAll(/installClient:\s*'([^']+)'/g)) {
    clients.push(match[1])
  }
  return clients
}

function read(file) {
  return fs.readFileSync(path.join(root, file), 'utf8')
}

const wizardSource = read('frontend/src/components/developers/DeveloperMcpWizard.jsx')
const developersSource = read('frontend/src/lib/developers.js')

const dashboardClients = parseDashboardInstallClients(wizardSource)

const installPlatforms = PLATFORMS.filter((platform) => platform !== 'skill')

function dashboardClientPlatform(client) {
  if (client === 'claude-code') return 'claude'
  return client
}

for (const platform of installPlatforms) {
  const actions = dryRunPlan(platform, { apiKey: 'lq_dev_test' })
  if (!actions.length || actions.some((line) => line.startsWith('Unknown platform'))) {
    fail(`dryRunPlan missing handler for platform: ${platform}`)
  }
}

for (const client of dashboardClients) {
  const platform = dashboardClientPlatform(client)
  if (!installPlatforms.includes(platform)) {
    fail(`dashboard client "${client}" has no matching CLI platform (expected one of: ${installPlatforms.join(', ')})`)
  }
}

const expectedDashboard = [
  'cursor',
  'claude',
  'claude-desktop',
  'copilot',
  'roo',
  'windsurf',
  'cline',
  'gemini',
]

for (const client of expectedDashboard) {
  if (!dashboardClients.includes(client)) {
    fail(`DeveloperMcpWizard missing installClient tab: ${client}`)
  }
}

const assetFiles = [
  'packages/joinquest/assets/skill/SKILL.md',
  'packages/joinquest/assets/plugin/.cursor-plugin/plugin.json',
  'packages/joinquest/assets/plugin/skills/joinquest-integration/SKILL.md',
]

for (const file of assetFiles) {
  if (!fs.existsSync(path.join(root, file))) {
    fail(`Missing bundled asset (run npm run sync-assets in packages/joinquest): ${file}`)
  }
}

if (!developersSource.includes('npx -y ${JOINQUEST_CLI_PACKAGE} install')) {
  fail('developers.js install commands should use npx joinquest install')
}

console.log(
  `OK: install parity — ${installPlatforms.length} CLI platforms, ${dashboardClients.length} dashboard tabs, assets present`,
)
