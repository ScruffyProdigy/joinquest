import { useEffect, useState } from 'react'
import { Link } from '../ui/link'
import AuthPanel from '../auth/AuthPanel'
import { useAuth } from '../auth/AuthProvider'
import { APP_NAME } from '../../lib/brand'
import { developerLandingHref, parseDeveloperLandingPath } from '../../lib/developers'
import { navigateTo } from '../../lib/usePathname'
import DeveloperAuthGate from './DeveloperAuthGate'
import DeveloperMcpWizard from './DeveloperMcpWizard'
import ReferenceGamesPanel from './ReferenceGamesPanel'
import RegisterGameForm from './RegisterGameForm'

function readLandingPathFromUrl() {
  return parseDeveloperLandingPath(window.location.search)
}

export default function DeveloperLandingPage() {
  const { user, loading } = useAuth()
  const [route, setRoute] = useState(() => readLandingPathFromUrl())
  // Which action is waiting on an account, so we can resume it after sign-in.
  const [gate, setGate] = useState(null)

  // Generating an API key and registering a game both need a durable identity.
  const needsAccount = !user || user.isGuest

  useEffect(() => {
    const syncFromUrl = () => setRoute(readLandingPathFromUrl())
    window.addEventListener('popstate', syncFromUrl)
    return () => window.removeEventListener('popstate', syncFromUrl)
  }, [])

  useEffect(() => {
    // Signing in without leaving the page drops the visitor back into what they were doing.
    if (gate && !needsAccount) {
      setRoute(gate)
      setGate(null)
    }
  }, [gate, needsAccount])

  function selectRoute(next) {
    if (next === 'ai' && needsAccount) {
      setGate('ai')
      return
    }
    setRoute(next)
    navigateTo(developerLandingHref(next), { replace: true })
  }

  function clearRoute() {
    setRoute(null)
    navigateTo(developerLandingHref(), { replace: true })
  }

  function dismissGate() {
    setGate(null)
  }

  if (gate) {
    return (
      <main className="app-shell developer-shell">
        {/* Opaque keys the backend maps back to this page after OAuth sign-in. */}
        <DeveloperAuthGate onBack={dismissGate} next={gate === 'ai' ? 'dev-ai' : 'dev-manual'} />
      </main>
    )
  }

  return (
    <main className="app-shell developer-shell">
      {loading || user ? <AuthPanel variant="developer" /> : null}

      <header className="app-header">
        <p className="developer-back">
          <Link href="/">
            ← Back to {APP_NAME}
          </Link>
        </p>
        <h1>Have an idea for a multiplayer game?</h1>
      </header>

      <section className="panel-card developer-intro">
        <p className="panel-copy">
          {APP_NAME} handles the boring stuff — player accounts, rooms, tables, matchmaking, finding
          players — so you can focus on building the game.
        </p>
        <p className="panel-copy">
          You&apos;ll need some basic web dev experience and a <strong>public URL</strong> where your
          game can run. That&apos;s it.
        </p>
        <p className="panel-copy">
          Your game won&apos;t show up for other players until <em>you</em> say it&apos;s ready (and
          we&apos;ve had a quick look). You&apos;re not committing to anything.
        </p>
      </section>

      <ReferenceGamesPanel />

      {route === null ? (
        <section className="developer-route-picker" aria-labelledby="developer-route-heading">
          <h2 id="developer-route-heading">How do you want to get started?</h2>
          <p className="panel-copy developer-route-picker__lead">
            Pick one path — most developers use an AI assistant (Cursor, Claude, Copilot, and more).
          </p>
          <div className="developer-route-picker__options">
            <button
              type="button"
              className="developer-route-card developer-route-card--recommended"
              onClick={() => selectRoute('ai')}
            >
              <span className="developer-route-card__badge">Recommended</span>
              <h3 className="developer-route-card__title">Connect an AI assistant</h3>
              <p className="developer-route-card__copy">
                Your agent can register the game, run integration checks, and save metadata from your
                editor — no browser cookies required.
              </p>
            </button>
            <button
              type="button"
              className="developer-route-card"
              onClick={() => selectRoute('manual')}
            >
              <h3 className="developer-route-card__title">Register in the browser</h3>
              <p className="developer-route-card__copy">
                Fill out the registration form yourself if you prefer not to use an AI assistant.
              </p>
            </button>
          </div>
        </section>
      ) : (
        <>
          <p className="developer-route-back">
            <Link onClick={clearRoute}>← Choose a different path</Link>
          </p>

          {route === 'ai' ? (
            <DeveloperMcpWizard alwaysExpanded />
          ) : (
            <section className="panel-card" aria-labelledby="register-game-heading">
              <h2 id="register-game-heading">Register your game</h2>
              <RegisterGameForm onAccountRequired={() => setGate('manual')} />
            </section>
          )}
        </>
      )}
    </main>
  )
}
