import { PLATFORMS } from './constants.js'
import { runInstall } from './install.js'

const HELP = `joinquest — install JoinQuest agent skill + MCP for your editor

Usage:
  npx joinquest install <platform> [options]
  npm create joinquest@latest -- --cursor

Platforms:
  cursor, claude, claude-desktop, copilot, roo, windsurf, cline, gemini, skill

Options:
  --dry-run           Show planned actions without writing files
  --plugin            Global plugin install (cursor or claude only)
  --api-key <key>     JoinQuest developer API key (or set JOINQUEST_API_KEY)

Examples:
  JOINQUEST_API_KEY=lq_dev_... npx joinquest install cursor
  npx joinquest install copilot --dry-run
  npx joinquest install cursor --plugin

API key: https://joinquest.cc/developers → Connect an AI assistant
MCP package: npx @joinquest/mcp-integration (same trust model as this installer)
`

export function parseInstallArgv(argv) {
  const args = [...argv]
  let dryRun = false
  let plugin = false
  let apiKey = process.env.JOINQUEST_API_KEY || ''

  const positional = []
  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i]
    if (arg === '--dry-run') {
      dryRun = true
    } else if (arg === '--plugin') {
      plugin = true
    } else if (arg === '--api-key') {
      apiKey = args[++i] || ''
    } else if (arg === '--help' || arg === '-h') {
      return { help: true }
    } else if (arg.startsWith('--')) {
      const legacy = arg.replace(/^--/, '')
      if (PLATFORMS.includes(legacy)) {
        positional.push(legacy)
      } else {
        throw new Error(`Unknown option: ${arg}`)
      }
    } else {
      positional.push(arg)
    }
  }

  return { dryRun, plugin, apiKey, positional }
}

export async function runCli(argv) {
  if (argv.length === 0 || argv[0] === '--help' || argv[0] === '-h') {
    console.log(HELP)
    return 0
  }

  const sub = argv[0]
  if (sub !== 'install') {
    if (PLATFORMS.includes(sub)) {
      return runCli(['install', ...argv])
    }
    console.error(`Unknown command: ${sub}\n`)
    console.log(HELP)
    return 1
  }

  const parsed = parseInstallArgv(argv.slice(1))
  if (parsed.help) {
    console.log(HELP)
    return 0
  }

  const platform = parsed.positional[0]
  if (!platform) {
    console.error('Missing platform. Example: npx joinquest install cursor\n')
    console.log(HELP)
    return 1
  }

  if (!PLATFORMS.includes(platform)) {
    throw new Error(`Unknown platform "${platform}". Choose: ${PLATFORMS.join(', ')}`)
  }

  await runInstall(platform, {
    dryRun: parsed.dryRun,
    plugin: parsed.plugin,
    apiKey: parsed.apiKey,
  })
  return 0
}

/** npm create joinquest — accepts --cursor or positional platform. */
export async function runCreateCli(argv) {
  const parsed = parseInstallArgv(argv)
  if (parsed.help) {
    console.log(HELP)
    return 0
  }
  const platform = parsed.positional[0] || 'cursor'
  if (!PLATFORMS.includes(platform)) {
    throw new Error(`Unknown platform "${platform}". Choose: ${PLATFORMS.join(', ')}`)
  }
  await runInstall(platform, {
    dryRun: parsed.dryRun,
    plugin: parsed.plugin,
    apiKey: parsed.apiKey,
  })
  return 0
}
