import { useEffect, useState } from 'react'
import SignInPanel from '../auth/SignInPanel'
import { useAuth } from '../auth/AuthProvider'
import { navigateBackToCatalog } from '../../lib/catalogNavigation'
import { gameDetailDescription, gameHeroUrl, gameTagChips } from '../../lib/gameCard'
import { fetchGameBySlug } from '../../lib/games'
import GameModesPanel from './GameModesPanel'
import GameShareButton from './GameShareButton'
import AppFooter from '../legal/AppFooter'

function GameDetailToolbar({ game, onBack = navigateBackToCatalog }) {
  return (
    <div className="game-detail__toolbar">
      <button type="button" className="game-detail__toolbar-btn" onClick={onBack}>
        ← Back
      </button>
      <GameShareButton game={game} />
    </div>
  )
}

function GameDetailPlaySection({
  game,
  authLoading,
  user,
  activeIntent,
  activeTableSeat,
  onQueueChange,
  onQueueJoined,
  onTableChange,
}) {
  if (authLoading) {
    return (
      <section className="game-detail__play">
        <p className="status-message" role="status">
          Checking session…
        </p>
      </section>
    )
  }

  if (user) {
    return (
      <section className="game-detail__play">
        <GameModesPanel
          game={game}
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          onQueueChange={onQueueChange}
          onQueueJoined={onQueueJoined}
          onTableChange={onTableChange}
          heading={null}
          variant="prominent"
        />
      </section>
    )
  }

  return (
    <section className="game-detail__play game-detail__sign-in">
      <h2 className="game-detail__section-title">Ready to play?</h2>
      <p className="panel-copy">Sign in to look for a group or create a private table.</p>
      <SignInPanel />
    </section>
  )
}

export default function GameDetailPage({
  slug,
  activeIntent,
  activeTableSeat,
  onQueueChange,
  onQueueJoined,
  onTableChange,
}) {
  const { user, loading: authLoading } = useAuth()
  const [game, setGame] = useState(null)
  const [status, setStatus] = useState('loading')
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setStatus('loading')
    setError('')

    fetchGameBySlug(slug, user?.id ?? '')
      .then((item) => {
        if (cancelled) {
          return
        }
        setGame(item)
        setStatus(item ? 'ready' : 'missing')
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        setGame(null)
        setStatus('error')
        setError(err.message || 'Could not load game')
      })

    return () => {
      cancelled = true
    }
  }, [slug, user?.id])

  if (status === 'loading') {
    return (
      <main className="app-shell game-detail">
        <p className="status-message" role="status">
          Loading game…
        </p>
      </main>
    )
  }

  if (status === 'error') {
    return (
      <main className="app-shell game-detail">
        <p className="status-message status-message-error" role="status">
          {error}
        </p>
        <button type="button" className="game-detail__toolbar-btn" onClick={navigateBackToCatalog}>
          ← Back to catalog
        </button>
      </main>
    )
  }

  if (status === 'missing' || !game) {
    return (
      <main className="app-shell game-detail">
        <h1>Game not found</h1>
        <p className="panel-copy">We could not find a game at this address.</p>
        <button type="button" className="game-detail__toolbar-btn" onClick={navigateBackToCatalog}>
          ← Back to catalog
        </button>
      </main>
    )
  }

  const tags = gameTagChips(game.tags)
  const description = gameDetailDescription(game)
  const screenshots = (game.screenshots ?? []).filter((url) => String(url).trim())

  return (
    <main className="app-shell game-detail">
      <div className="game-detail__inset">
        <GameDetailToolbar game={game} />
      </div>

      <div className="game-detail__hero-wrap">
        <img
          className="game-detail__hero"
          src={gameHeroUrl(game)}
          alt=""
          width={960}
          height={540}
          loading="eager"
        />
      </div>

      <div className="game-detail__inset">
        <header className="game-detail__header">
          <h1 className="game-detail__title">{game.name}</h1>
          {tags.length > 0 ? (
            <ul className="game-list-item__tags game-detail__tags" aria-label="Game tags">
              {tags.map((tag) => (
                <li key={tag} className="game-list-item__tag">
                  {tag}
                </li>
              ))}
            </ul>
          ) : null}
        </header>

        <GameDetailPlaySection
          game={game}
          authLoading={authLoading}
          user={user}
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          onQueueChange={onQueueChange}
          onQueueJoined={onQueueJoined}
          onTableChange={onTableChange}
        />

        {description ? <p className="game-detail__description">{description}</p> : null}

        {game.howToPlay ? (
          <section className="game-detail__section">
            <h2 className="game-detail__section-title">How to play</h2>
            <p className="game-detail__body">{game.howToPlay}</p>
          </section>
        ) : null}

        {game.tutorialUrl ? (
          <p className="game-detail__tutorial">
            <a className="auth-link" href={game.tutorialUrl} target="_blank" rel="noopener noreferrer">
              Open tutorial
            </a>
          </p>
        ) : null}

        {screenshots.length > 0 ? (
          <section className="game-detail__section">
            <h2 className="game-detail__section-title">Screenshots</h2>
            <ul className="game-detail__screenshots">
              {screenshots.map((url) => (
                <li key={url}>
                  <img className="game-detail__screenshot" src={url} alt="" loading="lazy" />
                </li>
              ))}
            </ul>
          </section>
        ) : null}
      </div>

      <AppFooter />
    </main>
  )
}
