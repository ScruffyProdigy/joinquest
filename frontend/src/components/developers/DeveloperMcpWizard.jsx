import { useEffect, useMemo, useState } from 'react'
import {
  buildClaudeMcpAddCommand,
  buildGeminiMcpAddCommand,
  buildInstallClaudePluginCommand,
  buildInstallCursorPluginCommand,
  buildInstallDevCommand,
  buildInstallDevInspectCommand,
  buildMcpServerConfig,
  createDeveloperApiKey,
  fetchMyDeveloperApiKeys,
  INSTALL_CLAUDE_PLUGIN_SCRIPT_GITHUB,
  INSTALL_CURSOR_PLUGIN_SCRIPT_GITHUB,
  INSTALL_DEV_SCRIPT_GITHUB,
  INSTALL_SETUP_MANIFEST_GITHUB,
  JOINQUEST_CLI_GITHUB,
  CURSOR_PLUGIN_GITHUB,
  developerLandingHref,
} from '../../lib/developers'
import { navigateTo } from '../../lib/usePathname'

const CLIENTS = [
  {
    id: 'cursor',
    label: 'Cursor',
    installClient: 'cursor',
    usesPlugin: true,
    configPath: '~/.cursor/mcp.json or .cursor/mcp.json',
  },
  {
    id: 'claude-code',
    label: 'Claude Code',
    installClient: 'claude',
    usesPlugin: true,
    configPath: '.mcp.json (created automatically by claude mcp add)',
  },
  {
    id: 'claude-desktop',
    label: 'Claude Desktop',
    installClient: 'claude-desktop',
    configPath: 'Settings → Developer → Edit Config (see manual setup below)',
  },
  {
    id: 'copilot',
    label: 'GitHub Copilot',
    installClient: 'copilot',
    configPath: '.vscode/mcp.json (uses servers, not mcpServers)',
  },
  {
    id: 'roo',
    label: 'Roo Code',
    installClient: 'roo',
    configPath: '.roo/mcp.json in your game repo',
  },
  {
    id: 'windsurf',
    label: 'Windsurf',
    installClient: 'windsurf',
    configPath: '~/.codeium/windsurf/mcp_config.json (global only)',
  },
  {
    id: 'cline',
    label: 'Cline',
    installClient: 'cline',
    configPath: 'Cline → MCP Servers → Configure (VS Code globalStorage)',
  },
  {
    id: 'gemini',
    label: 'Gemini CLI',
    installClient: 'gemini',
    configPath: '.gemini/settings.json in your game repo',
  },
  {
    id: 'chatgpt',
    label: 'ChatGPT',
    unsupported: true,
  },
]

const AGENT_VERIFY_PROMPT = 'List my JoinQuest games using the MCP tools.'
const AGENT_START_PROMPT = 'I want to create a game on JoinQuest'

const RESTART_STEPS = {
  cursor: [
    'Quit Cursor completely (Cmd+Q on Mac — closing the window isn’t enough).',
    'Reopen your game project folder.',
    'Start a fresh Agent chat.',
    `Ask: “${AGENT_VERIFY_PROMPT}”`,
    `Then: “${AGENT_START_PROMPT}”`,
  ],
  'claude-desktop': [
    'Quit Claude Desktop, then open it again.',
    'Start a new conversation.',
    'Check that joinquest-integration appears under MCP servers.',
    `Ask: “${AGENT_VERIFY_PROMPT}”`,
    `Then: “${AGENT_START_PROMPT}”`,
  ],
  'claude-code': [
    'Start a new Claude Code session in your game repo.',
    'Approve joinquest-integration when prompted.',
    `Ask: “${AGENT_VERIFY_PROMPT}”`,
    `Then: “${AGENT_START_PROMPT}”`,
  ],
  copilot: [
    'Restart VS Code (or reload the window).',
    'Open Agent mode in Copilot Chat.',
    'Approve joinquest-integration MCP tools when prompted.',
    `Ask: “${AGENT_VERIFY_PROMPT}”`,
    `Then: “${AGENT_START_PROMPT}”`,
  ],
  roo: [
    'Reload VS Code or start a new Roo Code chat in your game repo.',
    'Approve joinquest-integration when prompted.',
    `Ask: “${AGENT_VERIFY_PROMPT}”`,
    `Then: “${AGENT_START_PROMPT}”`,
  ],
  windsurf: [
    'Export JOINQUEST_API_KEY in your shell (the install script sets this up).',
    'Fully quit Windsurf (Cmd+Q), then reopen.',
    'Open Cascade and confirm joinquest-integration is connected.',
    `Ask: “${AGENT_VERIFY_PROMPT}”`,
    `Then: “${AGENT_START_PROMPT}”`,
  ],
  cline: [
    'Reload the Cline panel in VS Code.',
    'Open MCP Servers → confirm joinquest-integration is enabled.',
    `Ask: “${AGENT_VERIFY_PROMPT}”`,
    `Then: “${AGENT_START_PROMPT}”`,
  ],
  gemini: [
    'Start a new Gemini CLI session in your game repo (or run /mcp list to confirm the server).',
    'Approve joinquest-integration when prompted.',
    `Ask: “${AGENT_VERIFY_PROMPT}”`,
    `Then: “${AGENT_START_PROMPT}”`,
  ],
}

const PLATFORM_INSTALL_NOTES = {
  copilot: [
    'Adds .agents/skills/joinquest-integration/ plus .github/skills/joinquest-integration/.',
    'Writes .github/copilot-instructions.md (MCP-first — Copilot may truncate long skills).',
    'Merges joinquest-integration into .vscode/mcp.json.',
  ],
  roo: [
    'Adds .agents/skills/joinquest-integration/ and .roo/rules/joinquest-integration/.',
    'Creates .roo/mcp.json — commit it to share MCP setup with your team.',
  ],
  windsurf: [
    'Adds .agents/skills/joinquest-integration/ and .windsurf/rules/joinquest-integration/.',
    'Merges MCP into ~/.codeium/windsurf/mcp_config.json (global — not per-repo).',
  ],
  cline: [
    'Adds .agents/skills/joinquest-integration/, .cline/rules/joinquest-integration/, and .clinerules.',
    'Merges MCP into Cline’s cline_mcp_settings.json.',
  ],
  'claude-desktop': [
    'Adds .agents/skills/joinquest-integration/ — step-by-step instructions for your agent.',
    'Merges MCP into Claude Desktop’s config (macOS, Windows, or Linux).',
  ],
  gemini: [
    'Adds .agents/skills/joinquest-integration/ to your game repo.',
    'Runs gemini mcp add (project scope) or writes .gemini/settings.json.',
  ],
}

const INSTALL_GLOBAL_MCP =
  'curl -fsSL https://raw.githubusercontent.com/scruffyprodigy/playhub/main/scripts/install-joinquest-mcp.sh | sh'

function buildClaudeCodeConfig(base) {
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

function manualConfigCopyLabel(clientId) {
  if (clientId === 'claude-desktop') return 'Copy MCP config'
  if (clientId === 'copilot') return 'Copy Copilot JSON'
  if (clientId === 'windsurf') return 'Copy Windsurf JSON'
  return 'Copy MCP JSON'
}

function ClientTabs({ clientId, onChange }) {
  return (
    <div className="developer-mcp__tabs" role="tablist" aria-label="Which editor do you use?">
      {CLIENTS.map((item) => (
        <button
          key={item.id}
          type="button"
          role="tab"
          aria-selected={clientId === item.id}
          className={`developer-mcp__tab${clientId === item.id ? ' developer-mcp__tab--active' : ''}`}
          onClick={() => onChange(item.id)}
        >
          {item.label}
        </button>
      ))}
    </div>
  )
}

export default function DeveloperMcpWizard({ defaultExpanded = false, alwaysExpanded = false }) {
  const [expanded, setExpanded] = useState(defaultExpanded || alwaysExpanded)
  const [clientId, setClientId] = useState('cursor')
  const [apiKeys, setApiKeys] = useState([])
  const [apiKeyValue, setApiKeyValue] = useState('')
  const [showApiKey, setShowApiKey] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState('')

  useEffect(() => {
    let cancelled = false
    fetchMyDeveloperApiKeys()
      .then((keys) => {
        if (!cancelled) {
          setApiKeys(keys)
        }
      })
      .catch(() => {
        // ignore — user may not be signed in yet
      })
    return () => {
      cancelled = true
    }
  }, [])

  const activeKey = apiKeyValue.trim() || null
  const client = CLIENTS.find((item) => item.id === clientId) ?? CLIENTS[0]
  const unsupported = client.unsupported === true
  const restartSteps = RESTART_STEPS[clientId] ?? RESTART_STEPS.cursor
  const platformNotes = PLATFORM_INSTALL_NOTES[clientId] ?? []

  const configText = useMemo(() => {
    if (unsupported) return ''
    const base = buildMcpServerConfig({
      apiKey: activeKey ?? '<paste-api-key-here>',
      clientId,
    })
    const payload = clientId === 'claude-code' ? buildClaudeCodeConfig(base) : base
    return JSON.stringify(payload, null, 2)
  }, [activeKey, clientId, unsupported])

  const claudeAddCommand = useMemo(
    () =>
      buildClaudeMcpAddCommand({
        apiKey: activeKey ?? '<paste-api-key-here>',
      }),
    [activeKey],
  )

  const geminiAddCommand = useMemo(
    () =>
      buildGeminiMcpAddCommand({
        apiKey: activeKey ?? '<paste-api-key-here>',
      }),
    [activeKey],
  )

  const installDevCommand = useMemo(
    () =>
      buildInstallDevCommand({
        apiKey: activeKey ?? undefined,
        client: client.installClient,
      }),
    [activeKey, client.installClient],
  )

  const installCursorPluginCommand = useMemo(
    () =>
      buildInstallCursorPluginCommand({
        apiKey: activeKey ?? undefined,
      }),
    [activeKey],
  )

  const installClaudePluginCommand = useMemo(
    () =>
      buildInstallClaudePluginCommand({
        apiKey: activeKey ?? undefined,
      }),
    [activeKey],
  )

  const installPluginCommand =
    clientId === 'claude-code' ? installClaudePluginCommand : installCursorPluginCommand
  const installPluginScriptGithub =
    clientId === 'claude-code'
      ? INSTALL_CLAUDE_PLUGIN_SCRIPT_GITHUB
      : INSTALL_CURSOR_PLUGIN_SCRIPT_GITHUB
  const usesPlugin = client.usesPlugin === true

  const installInspectCommand = useMemo(
    () =>
      buildInstallDevInspectCommand({
        apiKey: activeKey ?? undefined,
        client: client.installClient,
      }),
    [activeKey, client.installClient],
  )

  async function handleGenerateKey() {
    setBusy(true)
    setError('')
    try {
      const result = await createDeveloperApiKey('Integration agent')
      setApiKeyValue(result.secret)
      setShowApiKey(true)
      setApiKeys((prev) => [result.apiKey, ...prev])
    } catch (err) {
      setError(err.message || 'Could not create API key.')
    } finally {
      setBusy(false)
    }
  }

  async function handleCopy(text, label) {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(label)
      setTimeout(() => setCopied(''), 2000)
    } catch {
      // ignore
    }
  }

  return (
    <section className="panel-card developer-mcp" aria-labelledby="mcp-heading">
      <div className="developer-mcp__header">
        <h2 id="mcp-heading">Connect an AI assistant</h2>
        {!alwaysExpanded ? (
          <button
            type="button"
            className="button-secondary developer-mcp__toggle"
            onClick={() => setExpanded((v) => !v)}
            aria-expanded={expanded}
          >
            {expanded ? 'Hide' : 'Show setup'}
          </button>
        ) : null}
      </div>
      {!(alwaysExpanded || expanded) ? (
        <p className="panel-copy">
          Hook up JoinQuest from Cursor, Claude, Copilot, Roo, Windsurf, or Cline so your agent can
          run checks and save metadata — no browser cookies required.
        </p>
      ) : (
        <>
          <div className="developer-mcp__client-picker">
            <ClientTabs clientId={clientId} onChange={setClientId} />
          </div>

          {unsupported ? (
            <div className="developer-mcp__unsupported">
              <div className="developer-mcp__info" role="note">
                <p>
                  <strong>ChatGPT isn&apos;t supported yet</strong>
                </p>
                <p>
                  JoinQuest connects through a local MCP server (<code>npx</code>, stdio). ChatGPT
                  connectors need a hosted HTTPS endpoint and OAuth — we can&apos;t wire that up
                  from your game repo today.
                </p>
              </div>

              <h3 className="developer-mcp__unsupported-heading">Use another assistant instead</h3>
              <p className="panel-copy">
                For the full agent workflow — integration checks, metadata, release — pick Cursor,
                Claude Code, Copilot, Roo, Windsurf, or Cline from the tabs above.
              </p>

              <h3 className="developer-mcp__unsupported-heading">Using ChatGPT today</h3>
              <p className="panel-copy">
                You can still register and manage your game in ChatGPT&apos;s browser:
              </p>
              <ol className="developer-mcp__steps">
                <li>
                  <a
                    className="auth-link"
                    href={developerLandingHref('manual')}
                    onClick={(event) => {
                      event.preventDefault()
                      navigateTo(developerLandingHref('manual'))
                    }}
                  >
                    Register in the browser
                  </a>{' '}
                  — fill out the registration form on joinquest.cc.
                </li>
                <li>After registration, use your game dashboard to run checks and request release.</li>
              </ol>
            </div>
          ) : (
            <>
              <p className="panel-copy">
                Three quick steps for <strong>{client.label}</strong>.
                {usesPlugin ? (
                  <>
                    {' '}
                    Install the JoinQuest {client.label} plugin — skill and MCP together, no
                    game-repo changes required.
                  </>
                ) : (
                  <> Run the install command from your game repo — we&apos;ll add the agent skill and connect MCP.</>
                )}
              </p>

              {clientId === 'copilot' ? (
                <div className="developer-mcp__info" role="note">
                  <p>
                    Copilot may truncate long skill files. The install script uses an MCP-first
                    setup — your agent should call{' '}
                    <code>joinquest_integration_get_agent_playbook</code> rather than relying on
                    reading the full playbook from disk.
                  </p>
                </div>
              ) : null}

              {clientId === 'windsurf' ? (
                <div className="developer-mcp__info" role="note">
                  <p>
                    Windsurf MCP is <strong>global only</strong> (~/.codeium/windsurf/). Export{' '}
                    <code>JOINQUEST_API_KEY</code> in your shell before launching Windsurf.
                  </p>
                </div>
              ) : null}

              <div className="developer-mcp__keys">
                <h3>1. Get an API key</h3>
                <p className="panel-copy">
                  This tells JoinQuest it&apos;s your agent calling. Paste a saved key below, or
                  generate a new one — either way, step 2 fills in the install command for you.
                </p>
                {apiKeys.length > 0 ? (
                  <div className="developer-mcp__info" role="note">
                    <p>
                      You already have a key (<code>{apiKeys[0].keyPrefix}…</code>
                      {apiKeys[0].lastUsedAt ? ', used recently' : ', not used yet'}). Paste it if
                      you saved it, or generate a new one.
                    </p>
                  </div>
                ) : null}
                <div className="developer-form__field developer-mcp__key-field">
                  <label htmlFor="developer-mcp-api-key">Your API key</label>
                  <div className="developer-mcp__key-input-row">
                    <input
                      id="developer-mcp-api-key"
                      type={showApiKey ? 'text' : 'password'}
                      autoComplete="off"
                      spellCheck={false}
                      placeholder="lq_dev_…"
                      value={apiKeyValue}
                      onChange={(event) => setApiKeyValue(event.target.value)}
                    />
                    <button
                      type="button"
                      className="button-secondary"
                      onClick={() => setShowApiKey((value) => !value)}
                      disabled={!apiKeyValue}
                    >
                      {showApiKey ? 'Hide' : 'Show'}
                    </button>
                  </div>
                  <p className="developer-form__hint">
                    We only use this in your browser to build the copy-paste commands below — nothing
                    gets sent until you run them.
                  </p>
                </div>
                <div className="developer-mcp__key-actions">
                  <button
                    type="button"
                    className="button-primary"
                    disabled={busy}
                    onClick={() => void handleGenerateKey()}
                  >
                    {busy ? 'Generating…' : 'Generate new API key'}
                  </button>
                  {activeKey ? (
                    <button
                      type="button"
                      className="button-secondary"
                      onClick={() => void handleCopy(activeKey, 'key')}
                    >
                      {copied === 'key' ? 'Copied' : 'Copy API key'}
                    </button>
                  ) : null}
                </div>
              </div>

              <div className="developer-mcp__install">
                <h3>
                  {usesPlugin ? `2. Install the ${client.label} plugin` : '2. Install the skill and MCP'}
                </h3>
                {usesPlugin ? (
                  <>
                    <p className="panel-copy">
                      Run this once on your machine — it installs skill + MCP for {client.label}.
                      {activeKey ? (
                        <> Your key is already in the first line — copy and run as-is.</>
                      ) : (
                        <>
                          {' '}
                          Paste or generate your key in step 1 first — we&apos;ll fill in{' '}
                          <code>JOINQUEST_API_KEY</code> for you.
                        </>
                      )}
                    </p>
                    <pre className="developer-mcp__config">{installPluginCommand}</pre>
                    <button
                      type="button"
                      className="button-secondary"
                      onClick={() => void handleCopy(installPluginCommand, 'install-plugin')}
                    >
                      {copied === 'install-plugin' ? 'Copied' : 'Copy plugin install command'}
                    </button>
                    <p className="panel-copy developer-mcp__what-it-does">What the plugin includes:</p>
                    <ul className="developer-mcp__explainer">
                      <li>
                        <strong>joinquest-integration</strong> agent skill — phased playbook for
                        discovery, API, checks, and release.
                      </li>
                      <li>
                        MCP server via <code>npx @joinquest/mcp-integration</code> (calls joinquest.cc
                        when tools run).
                      </li>
                    </ul>
                    <p className="panel-copy">
                      <a className="auth-link" href={CURSOR_PLUGIN_GITHUB} target="_blank" rel="noreferrer">
                        View plugin source on GitHub
                      </a>{' '}
                      ·{' '}
                      <a
                        className="auth-link"
                        href={installPluginScriptGithub}
                        target="_blank"
                        rel="noreferrer"
                      >
                        Read the install script
                      </a>
                    </p>
                    <details className="developer-mcp__details">
                      <summary>Alternative: curl install into your game repo</summary>
                      <p className="panel-copy">
                        Adds <code>.agents/skills/joinquest-integration/</code> to your project
                        {clientId === 'cursor' ? (
                          <> and wires MCP into <code>.cursor/mcp.json</code></>
                        ) : (
                          <> and runs <code>claude mcp add</code> for this project</>
                        )}
                        .
                      </p>
                      <pre className="developer-mcp__config">{installDevCommand}</pre>
                      <button
                        type="button"
                        className="button-secondary"
                        onClick={() => void handleCopy(installDevCommand, 'install-main')}
                      >
                        {copied === 'install-main' ? 'Copied' : 'Copy game-repo install command'}
                      </button>
                    </details>
                  </>
                ) : (
                  <>
                    <p className="panel-copy">
                      From your <strong>game repo root</strong>, run the command below.
                      {activeKey ? (
                        <> Your key is already in the first line — copy and run as-is.</>
                      ) : (
                        <>
                          {' '}
                          Paste or generate your key in step 1 first — we&apos;ll fill in{' '}
                          <code>JOINQUEST_API_KEY</code> for you.
                        </>
                      )}
                    </p>
                    <pre className="developer-mcp__config">{installDevCommand}</pre>
                    <button
                      type="button"
                      className="button-secondary"
                      onClick={() => void handleCopy(installDevCommand, 'install-main')}
                    >
                      {copied === 'install-main' ? 'Copied' : 'Copy install command'}
                    </button>
                    <p className="panel-copy developer-mcp__what-it-does">What the script does:</p>
                    <ul className="developer-mcp__explainer">
                      <li>
                        Adds <code>.agents/skills/joinquest-integration/</code> — step-by-step
                        instructions for your agent.
                      </li>
                      {platformNotes.map((note) => (
                        <li key={note}>{note}</li>
                      ))}
                      <li>
                        MCP runs via <code>npx @joinquest/mcp-integration</code> when your agent
                        needs it.
                      </li>
                    </ul>
                    <p className="panel-copy">
                      <a className="auth-link" href={JOINQUEST_CLI_GITHUB} target="_blank" rel="noreferrer">
                        joinquest npm package
                      </a>{' '}
                      on GitHub ·{' '}
                      <a className="auth-link" href={INSTALL_SETUP_MANIFEST_GITHUB} target="_blank" rel="noreferrer">
                        setup overview
                      </a>
                    </p>
                    <details className="developer-mcp__details">
                      <summary>Preview install (--dry-run)</summary>
                      <p className="panel-copy">
                        Shows what would be written without changing your project. Same package as MCP (
                        <code>npx joinquest</code>).
                      </p>
                      <pre className="developer-mcp__config">{installInspectCommand}</pre>
                      <button
                        type="button"
                        className="button-secondary"
                        onClick={() => void handleCopy(installInspectCommand, 'inspect')}
                      >
                        {copied === 'inspect' ? 'Copied' : 'Copy review-first commands'}
                      </button>
                    </details>
                  </>
                )}
                <details className="developer-mcp__details">
                  <summary>Offline or pinned install</summary>
                  <p className="panel-copy">Optional — if you don&apos;t want to use npx each time:</p>
                  <pre className="developer-mcp__config">{INSTALL_GLOBAL_MCP}</pre>
                </details>
              </div>

              <div className="developer-mcp__client">
                <h3>3. Restart and try it</h3>
                <p className="panel-copy">
                  {client.label} needs a full restart to pick up MCP. Then check that your agent can
                  reach JoinQuest:
                </p>
                <ol className="developer-mcp__steps">
                  {restartSteps.map((step) => (
                    <li key={step}>{step}</li>
                  ))}
                </ol>
                <div className="developer-mcp__quick-copy-actions">
                  <button
                    type="button"
                    className="button-primary"
                    onClick={() => void handleCopy(AGENT_START_PROMPT, 'start-prompt')}
                  >
                    {copied === 'start-prompt' ? 'Copied' : 'Copy start prompt'}
                  </button>
                </div>
                <p className="panel-copy">
                  From there, the agent skill walks you through discovery, integration checks, and
                  release. You approve anything that goes live.
                </p>
              </div>

              <details className="developer-mcp__details developer-mcp__manual">
                <summary>Manual setup (skip the script)</summary>
                <p className="panel-copy">
                  Prefer to paste config yourself?{' '}
                  {clientId === 'claude-desktop' ? (
                    <>
                      In Claude Desktop: <strong>Settings → Developer → Edit Config</strong>. Or edit
                      the file directly (macOS:{' '}
                      <code>~/Library/Application Support/Claude/claude_desktop_config.json</code>
                      ).
                    </>
                  ) : (
                    <>
                      File: <code>{client.configPath}</code>
                    </>
                  )}
                </p>
                {clientId === 'claude-code' ? (
                  <>
                    <pre className="developer-mcp__config">{claudeAddCommand}</pre>
                    <button
                      type="button"
                      className="button-secondary"
                      onClick={() => void handleCopy(claudeAddCommand, 'claude-add')}
                    >
                      {copied === 'claude-add' ? 'Copied' : 'Copy Claude command'}
                    </button>
                    <details className="developer-mcp__details">
                      <summary>Manual .mcp.json instead</summary>
                      <pre className="developer-mcp__config">{configText}</pre>
                      <button
                        type="button"
                        className="button-secondary"
                        onClick={() => void handleCopy(configText, 'config-manual')}
                      >
                        {copied === 'config-manual' ? 'Copied' : 'Copy .mcp.json'}
                      </button>
                    </details>
                  </>
                ) : clientId === 'gemini' ? (
                  <>
                    <pre className="developer-mcp__config">{geminiAddCommand}</pre>
                    <button
                      type="button"
                      className="button-secondary"
                      onClick={() => void handleCopy(geminiAddCommand, 'gemini-add')}
                    >
                      {copied === 'gemini-add' ? 'Copied' : 'Copy Gemini command'}
                    </button>
                    <details className="developer-mcp__details">
                      <summary>Manual settings.json instead</summary>
                      <pre className="developer-mcp__config">{configText}</pre>
                      <button
                        type="button"
                        className="button-secondary"
                        onClick={() => void handleCopy(configText, 'config-manual')}
                      >
                        {copied === 'config-manual' ? 'Copied' : 'Copy settings.json'}
                      </button>
                    </details>
                  </>
                ) : (
                  <>
                    <pre className="developer-mcp__config">{configText}</pre>
                    <button
                      type="button"
                      className="button-secondary"
                      onClick={() => void handleCopy(configText, 'config')}
                    >
                      {copied === 'config' ? 'Copied' : manualConfigCopyLabel(clientId)}
                    </button>
                  </>
                )}
              </details>
            </>
          )}

          {error ? (
            <p className="status-message status-message-error" role="alert">
              {error}
            </p>
          ) : null}
        </>
      )}
    </section>
  )
}
