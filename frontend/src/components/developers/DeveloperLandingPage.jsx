import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import AuthPanel from '../auth/AuthPanel'
import { APP_NAME } from '../../lib/brand'
import { developerLandingHref, parseDeveloperLandingPath } from '../../lib/developers'
import { navigateTo } from '../../lib/usePathname'
import DeveloperMcpWizard from './DeveloperMcpWizard'
import ReferenceGamesPanel from './ReferenceGamesPanel'
import RegisterGameForm from './RegisterGameForm'

function readLandingPathFromUrl() {
  return parseDeveloperLandingPath(window.location.search)
}

export default function DeveloperLandingPage() {
  const [route, setRoute] = useState(() => readLandingPathFromUrl())

  useEffect(() => {
    const syncFromUrl = () => setRoute(readLandingPathFromUrl())
    window.addEventListener('popstate', syncFromUrl)
    return () => window.removeEventListener('popstate', syncFromUrl)
  }, [])

  function selectRoute(next) {
    setRoute(next)
    navigateTo(developerLandingHref(next), { replace: true })
  }

  function clearRoute() {
    setRoute(null)
    navigateTo(developerLandingHref(), { replace: true })
  }

  return (
    <main className="app-shell developer-shell">
      <AuthPanel variant="developer" />

      <header className="app-header">
        <p className="developer-back">
          <a className="auth-link" href="/">
            ← Back to {APP_NAME}
          </a>
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
            <Button type="button" variant="link" onClick={clearRoute}>
              ← Choose a different path
            </Button>
          </p>

          {route === 'ai' ? (
            <DeveloperMcpWizard alwaysExpanded />
          ) : (
            <section className="panel-card" aria-labelledby="register-game-heading">
              <h2 id="register-game-heading">Register your game</h2>
              <RegisterGameForm />
            </section>
          )}
        </>
      )}
    </main>
  )
}
