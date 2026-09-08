import { useEffect, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { navigateBackToCatalog } from '../../lib/catalogNavigation'
import { gameDetailDescription, gameHeroUrl, gameTagChips } from '../../lib/gameCard'
import { fetchGameBySlug } from '../../lib/games'
import { accentColorFor } from '../../lib/gameAccent'
import GameModesPanel from './GameModesPanel'
import GameShareButton from './GameShareButton'
import AppFooter from '../legal/AppFooter'
import { Button } from '../ui/button'

function GameDetailToolbar({ game, onBack = navigateBackToCatalog }) {
  return (
    <div className="mb-3 flex items-center justify-between gap-3">
      <Button type="button" variant="ghost" size="sm" onClick={onBack}>
        ← Back
      </Button>
      <GameShareButton game={game} />
    </div>
  )
}

function GameDetailPlaySection({
  game,
  authLoading,
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

  // A visitor with no session sees the same play options as anyone else. The
  // name and avatar are collected when they act on one of them, and signing in
  // instead is offered inside that prompt — so the page stays a game page
  // rather than a sign-in wall.
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
      <main className="min-h-screen bg-background px-6 py-10 text-foreground">
        <p className="font-sans text-muted-foreground" role="status">
          Loading game…
        </p>
      </main>
    )
  }

  if (status === 'error') {
    return (
      <main className="min-h-screen bg-background px-6 py-10 text-foreground">
        <p className="font-sans text-destructive" role="status">
          {error}
        </p>
        <Button type="button" variant="ghost" size="sm" className="mt-3" onClick={navigateBackToCatalog}>
          ← Back to catalog
        </Button>
      </main>
    )
  }

  if (status === 'missing' || !game) {
    return (
      <main className="min-h-screen bg-background px-6 py-10 text-foreground">
        <h1 className="font-heading text-2xl font-bold">Game not found</h1>
        <p className="font-sans text-muted-foreground">We could not find a game at this address.</p>
        <Button type="button" variant="ghost" size="sm" className="mt-3" onClick={navigateBackToCatalog}>
          ← Back to catalog
        </Button>
      </main>
    )
  }

  const tags = gameTagChips(game.tags)
  const description = gameDetailDescription(game)
  const screenshots = (game.screenshots ?? []).filter((url) => String(url).trim())

  return (
    <main className="min-h-screen bg-background pb-8 text-foreground">
      <div className="px-6 pt-4">
        <GameDetailToolbar game={game} />
      </div>

      <div className="relative aspect-video w-full overflow-hidden">
        <img
          data-testid="game-detail-hero"
          className="h-full w-full object-cover"
          src={gameHeroUrl(game)}
          alt=""
          width={960}
          height={540}
          loading="eager"
        />
        <div
          className="pointer-events-none absolute inset-0"
          style={{ background: accentColorFor(game.slug, game.accentColor).heroScrim }}
        />
        <div className="absolute inset-x-0 bottom-0 flex flex-col gap-2 p-6">
          {tags.length > 0 ? (
            <ul className="flex flex-wrap gap-1" aria-label="Game tags">
              {tags.map((tag) => (
                <li
                  key={tag}
                  className="rounded-full px-2 py-1 text-[11px] font-semibold backdrop-blur-[6px]"
                  style={{ backgroundColor: 'rgba(0,0,0,0.42)', color: 'rgba(255,255,255,0.92)' }}
                >
                  {tag}
                </li>
              ))}
            </ul>
          ) : null}
          <h1 className="font-heading text-2xl font-bold text-white sm:text-3xl">{game.name}</h1>
        </div>
      </div>

      <div className="flex flex-col gap-5 px-6 pt-5">
        <GameDetailPlaySection
          game={game}
          authLoading={authLoading}
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          onQueueChange={onQueueChange}
          onQueueJoined={onQueueJoined}
          onTableChange={onTableChange}
        />

        {description ? (
          <p className="font-sans text-base leading-relaxed text-muted-foreground">{description}</p>
        ) : null}

        {game.howToPlay ? (
          <section>
            <h2 className="font-heading text-lg font-semibold">How to play</h2>
            <p className="mt-2 whitespace-pre-wrap font-sans leading-relaxed text-muted-foreground">
              {game.howToPlay}
            </p>
          </section>
        ) : null}

        {game.tutorialUrl ? (
          <p>
            <a className="font-sans text-primary underline-offset-4 hover:underline" href={game.tutorialUrl} target="_blank" rel="noopener noreferrer">
              Open tutorial
            </a>
          </p>
        ) : null}

        {screenshots.length > 0 ? (
          <section>
            <h2 className="font-heading text-lg font-semibold">Screenshots</h2>
            <ul className="mt-2 grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-3">
              {screenshots.map((url) => (
                <li key={url}>
                  <img className="block w-full rounded-lg border border-border" src={url} alt="" loading="lazy" />
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
