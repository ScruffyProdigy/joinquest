import { useCallback, useEffect, useMemo, useState } from 'react'
import { Button } from '../ui/button'
import { APP_NAME } from '../../lib/brand'
import {
  canRequestPublicRelease,
  checkFixHint,
  checkSectionTitle,
  checkStatusLabel,
  connectMyGame,
  defaultModeForMyGame,
  fetchDeveloperIntegrationGuide,
  fetchMyGame,
  fetchMyGameCredentials,
  requestPublicRelease,
  rotateMyGameWebhookSecret,
  runMyGameChecks,
  syncMyGameManifest,
  visibilityLabel,
} from '../../lib/developers'
import { createPrivateTable } from '../../lib/tables'
import { useActiveRoom } from '../rooms/ActiveRoomProvider'
import DeveloperCatalogMetadata from './DeveloperCatalogMetadata'
import DeveloperMcpWizard from './DeveloperMcpWizard'
import DeveloperNextSteps from './DeveloperNextSteps'
import DeveloperProvisionExample from './DeveloperProvisionExample'

function groupChecks(checks) {
  const groups = new Map()
  for (const check of checks ?? []) {
    const title = checkSectionTitle(check.checkId)
    if (!groups.has(title)) {
      groups.set(title, [])
    }
    groups.get(title).push(check)
  }
  return [...groups.entries()]
}

function CredentialField({ label, value }) {
  const [revealed, setRevealed] = useState(false)

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value)
    } catch {
      // ignore
    }
  }

  return (
    <div className="developer-credential">
      <span className="developer-credential__label">{label}</span>
      <code className="developer-credential__value">{revealed ? value : '••••••••••••••••'}</code>
      <div className="developer-credential__actions">
        <Button type="button" variant="secondary" onClick={() => setRevealed((v) => !v)}>
          {revealed ? 'Hide' : 'Reveal'}
        </Button>
        <Button type="button" variant="secondary" onClick={() => void handleCopy()}>
          Copy
        </Button>
      </div>
    </div>
  )
}

export default function DeveloperDashboard({ gameId }) {
  const { refresh: refreshRoom, openRoom } = useActiveRoom()
  const [game, setGame] = useState(null)
  const [credentials, setCredentials] = useState(null)
  const [guide, setGuide] = useState('')
  const [status, setStatus] = useState('loading')
  const [checksBusy, setChecksBusy] = useState(false)
  const [syncBusy, setSyncBusy] = useState(false)
  const [connectBusy, setConnectBusy] = useState(false)
  const [rotateBusy, setRotateBusy] = useState(false)
  const [tableBusy, setTableBusy] = useState(false)
  const [releaseBusy, setReleaseBusy] = useState(false)
  const [actionError, setActionError] = useState('')
  const [actionInfo, setActionInfo] = useState('')
  const [apiBaseUrlDraft, setApiBaseUrlDraft] = useState('')

  const load = useCallback(async () => {
    setStatus('loading')
    try {
      const [gameData, creds, guideText] = await Promise.all([
        fetchMyGame(gameId),
        fetchMyGameCredentials(gameId),
        fetchDeveloperIntegrationGuide(),
      ])
      setGame(gameData)
      setCredentials(creds)
      setGuide(guideText)
      setApiBaseUrlDraft(gameData?.apiBaseUrl ?? '')
      setStatus(gameData ? 'ready' : 'missing')
    } catch {
      setStatus('error')
    }
  }, [gameId])

  useEffect(() => {
    void load()
  }, [load])

  const checkGroups = useMemo(() => groupChecks(game?.integrationChecks), [game?.integrationChecks])
  const defaultMode = useMemo(() => defaultModeForMyGame(game), [game])
  const canTestTable = game?.visibility === 'PRIVATE_TESTING' || game?.visibility === 'PENDING_REVIEW'
  const canEditApiUrl = game?.visibility === 'DRAFT' || game?.visibility === 'PRIVATE_TESTING'
  const showRelease = canRequestPublicRelease(game)
  const showCatalog = game?.visibility !== 'DRAFT'
  const showCredentials = Boolean(credentials)

  async function handleRunChecks() {
    setChecksBusy(true)
    setActionError('')
    setActionInfo('')
    try {
      const checks = await runMyGameChecks(gameId)
      setGame((prev) => (prev ? { ...prev, integrationChecks: checks } : prev))
    } catch (err) {
      setActionError(err.message || 'Could not run checks.')
    } finally {
      setChecksBusy(false)
    }
  }

  async function handleSyncManifest() {
    setSyncBusy(true)
    setActionError('')
    setActionInfo('')
    try {
      const result = await syncMyGameManifest(gameId)
      if (result.connectError) {
        setActionError(result.connectError)
      } else {
        setActionInfo(
          result.changed
            ? 'Game modes updated from your API.'
            : 'Manifest already up to date — no changes.',
        )
      }
      if (result.game) {
        setGame(result.game)
        setApiBaseUrlDraft(result.game.apiBaseUrl ?? '')
      }
    } catch (err) {
      setActionError(err.message || 'Could not sync game modes.')
    } finally {
      setSyncBusy(false)
    }
  }

  async function handleConnectApi(event) {
    event.preventDefault()
    setConnectBusy(true)
    setActionError('')
    setActionInfo('')
    try {
      const trimmed = apiBaseUrlDraft.trim()
      const input = { gameId }
      if (trimmed && trimmed !== (game?.apiBaseUrl ?? '')) {
        input.apiBaseUrl = trimmed
      }
      const result = await connectMyGame(input)
      if (result.connectError) {
        setActionError(result.connectError)
      } else {
        setActionInfo(
          result.connected
            ? 'Connected — modes synced from your API.'
            : 'Connect finished without an error, but the API was not connected.',
        )
      }
      if (result.game) {
        setGame(result.game)
        setApiBaseUrlDraft(result.game.apiBaseUrl ?? '')
      }
    } catch (err) {
      setActionError(err.message || 'Could not connect API.')
    } finally {
      setConnectBusy(false)
    }
  }

  async function handleRotateWebhook() {
    if (
      !window.confirm(
        'Rotate the webhook secret? The previous secret will stop working immediately. Update your game server env after this.',
      )
    ) {
      return
    }
    setRotateBusy(true)
    setActionError('')
    setActionInfo('')
    try {
      const creds = await rotateMyGameWebhookSecret(gameId)
      setCredentials(creds)
      setActionInfo('Webhook secret rotated — copy the new secret below and update your game.')
    } catch (err) {
      setActionError(err.message || 'Could not rotate webhook secret.')
    } finally {
      setRotateBusy(false)
    }
  }

  async function handleRequestRelease() {
    setReleaseBusy(true)
    setActionError('')
    setActionInfo('')
    try {
      const updated = await requestPublicRelease(gameId)
      setGame((prev) => (prev ? { ...prev, visibility: updated.visibility } : prev))
    } catch (err) {
      setActionError(err.message || 'Could not request public release.')
    } finally {
      setReleaseBusy(false)
    }
  }

  async function handleCreateTestTable() {
    if (!defaultMode?.id) {
      setActionError('No active mode found — connect your API and sync game modes first.')
      return
    }
    setTableBusy(true)
    setActionError('')
    setActionInfo('')
    try {
      await createPrivateTable(game.id, defaultMode.id)
      await refreshRoom()
      openRoom()
    } catch (err) {
      setActionError(err.message || 'Could not create test table.')
    } finally {
      setTableBusy(false)
    }
  }

  if (status === 'loading') {
    return (
      <main className="app-shell developer-shell">
        <p className="status-message" role="status">
          Loading dashboard…
        </p>
      </main>
    )
  }

  if (status !== 'ready' || !game) {
    return (
      <main className="app-shell developer-shell">
        <h1>Game not found</h1>
        <a className="auth-link" href="/developers">
          Back to developers
        </a>
      </main>
    )
  }

  return (
    <main className="app-shell developer-shell">
      <header className="app-header">
        <p className="developer-back">
          <a className="auth-link" href="/developers">
            ← Developers
          </a>
          {' · '}
          <a className="auth-link" href="/">
            {APP_NAME}
          </a>
        </p>
        <h1>{game.name}</h1>
        <p className="tagline">{visibilityLabel(game.visibility)}</p>
      </header>

      <DeveloperMcpWizard />

      <DeveloperNextSteps game={game} />

      <section className="panel-card developer-actions">
        <h2>Actions</h2>
        <div className="developer-actions__row">
          <button
            type="button"
            className="button-primary"
            disabled={checksBusy}
            onClick={() => void handleRunChecks()}
          >
            {checksBusy ? 'Running checks…' : 'Run all checks'}
          </button>
          <button
            type="button"
            className="button-secondary"
            disabled={syncBusy}
            onClick={() => void handleSyncManifest()}
          >
            {syncBusy ? 'Syncing…' : 'Resync game modes'}
          </button>
          {canTestTable ? (
            <button
              type="button"
              className="button-secondary"
              disabled={tableBusy}
              onClick={() => void handleCreateTestTable()}
            >
              {tableBusy ? 'Creating table…' : 'Create test table'}
            </button>
          ) : null}
          {showRelease ? (
            <button
              type="button"
              className="button-secondary"
              disabled={releaseBusy}
              onClick={() => void handleRequestRelease()}
            >
              {releaseBusy ? 'Submitting…' : 'Request public release'}
            </button>
          ) : null}
        </div>

        {canEditApiUrl ? (
          <form className="developer-form developer-connect-form" onSubmit={(event) => void handleConnectApi(event)}>
            <h3 className="developer-actions__subheading">Connect / update API URL</h3>
            <p className="panel-copy">
              Public HTTPS origin for your game API. Use this to retry a failed connect or move from
              staging to production. Cannot change while pending review or public.
            </p>
            <div className="developer-form__field">
              <label htmlFor="dev-api-base-url">API base URL</label>
              <input
                id="dev-api-base-url"
                type="url"
                value={apiBaseUrlDraft}
                onChange={(event) => setApiBaseUrlDraft(event.target.value)}
                placeholder="https://mygame.example.com"
                required
              />
            </div>
            <Button type="submit" variant="secondary" disabled={connectBusy}>
              {connectBusy ? 'Connecting…' : 'Connect API'}
            </Button>
          </form>
        ) : (
          <p className="panel-copy">
            API base URL: <code>{game.apiBaseUrl}</code>
            {game.visibility === 'PENDING_REVIEW' || game.visibility === 'PUBLIC'
              ? ' (locked while pending review or public)'
              : null}
          </p>
        )}

        {game?.visibility === 'PENDING_REVIEW' ? (
          <p className="panel-copy" role="status">
            Your game is pending review — we&apos;ll email you at {game.contactEmail} when it&apos;s approved.
          </p>
        ) : null}
        {actionInfo ? (
          <p className="status-message" role="status">
            {actionInfo}
          </p>
        ) : null}
        {actionError ? (
          <p className="status-message status-message-error" role="alert">
            {actionError}
          </p>
        ) : null}
        <p className="panel-copy developer-footnote">
          Registering is free and instant. Showing up in the main {APP_NAME} catalog means a quick
          review — mostly so names aren&apos;t spam or obvious IP issues.
        </p>
      </section>

      <section className="panel-card" aria-labelledby="checklist-heading">
        <h2 id="checklist-heading">Integration checklist</h2>
        {checkGroups.length === 0 ? (
          <p className="panel-copy">No checks run yet. Hit &ldquo;Run all checks&rdquo; to start.</p>
        ) : (
          checkGroups.map(([section, checks]) => (
            <div key={section} className="developer-check-section">
              <h3>{section}</h3>
              <ul className="developer-check-list">
                {checks.map((check) => (
                  <li
                    key={check.checkId}
                    className={`developer-check developer-check--${check.status.toLowerCase()}`}
                  >
                    <div className="developer-check__header">
                      <span className="developer-check__id">{check.checkId}</span>
                      <span className="developer-check__status">{checkStatusLabel(check.status)}</span>
                    </div>
                    {check.message ? <p className="developer-check__message">{check.message}</p> : null}
                    {check.status === 'FAIL' ? (
                      <p className="developer-check__fix">{checkFixHint(check.checkId)}</p>
                    ) : null}
                    {check.detail ? (
                      <details className="developer-check__detail">
                        <summary>Details</summary>
                        <pre>{check.detail}</pre>
                      </details>
                    ) : null}
                  </li>
                ))}
              </ul>
            </div>
          ))
        )}
      </section>

      {showCredentials ? (
        <section className="panel-card" aria-labelledby="credentials-heading">
          <h2 id="credentials-heading">Integration credentials</h2>
          <CredentialField label="Service token" value={credentials.serviceToken} />
          <CredentialField label="Webhook secret" value={credentials.webhookSecret} />
          <p className="panel-copy">
            The service token is derived from your game id (not rotatable). Rotate the webhook secret
            if it leaks — the previous secret stops working immediately.
          </p>
          <button
            type="button"
            className="button-secondary"
            disabled={rotateBusy}
            onClick={() => void handleRotateWebhook()}
          >
            {rotateBusy ? 'Rotating…' : 'Rotate webhook secret'}
          </button>
        </section>
      ) : null}

      {showCredentials ? (
        <DeveloperProvisionExample game={game} credentials={credentials} />
      ) : null}

      {showCatalog ? <DeveloperCatalogMetadata game={game} onSaved={setGame} /> : null}

      {guide ? (
        <details className="panel-card developer-guide">
          <summary>Integration guide (full reference)</summary>
          <pre className="developer-guide__body">{guide}</pre>
        </details>
      ) : null}
    </main>
  )
}
