import { useEffect, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { fetchGames, filterGamesBySearch } from '../../lib/games'
import GameCard from './GameCard'
import { Input } from '../ui/input'
import {
  GAMES_HEADING,
  GAMES_INTRO,
  GAMES_SEARCH_EMPTY,
  GAMES_SEARCH_EMPTY_HINT,
  GAMES_SEARCH_LABEL,
  GAMES_SEARCH_PLACEHOLDER,
} from '../../lib/playerCopy'

export default function GameLobby() {
  const { user, loading: authLoading } = useAuth()
  const [games, setGames] = useState([])
  const [status, setStatus] = useState('idle')
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')

  // The catalog is public. Wait for the session check so the query can carry a
  // player id when there is one, but render for signed-out visitors either way.
  useEffect(() => {
    if (authLoading) {
      return
    }

    let cancelled = false
    setStatus('loading')
    setError('')

    fetchGames(user?.id ?? '')
      .then((items) => {
        if (cancelled) {
          return
        }
        setGames(items)
        setStatus('ready')
      })
      .catch((err) => {
        if (cancelled) {
          return
        }
        setGames([])
        setStatus('error')
        setError(err.message || 'Could not load games')
      })

    return () => {
      cancelled = true
    }
  }, [authLoading, user?.id])

  // Only show search once there is a catalog to search through.
  const hasCatalog = status === 'ready' && games.length > 0
  const visibleGames = filterGamesBySearch(games, search)

  return (
    <section className="game-lobby panel-card" aria-labelledby="games-heading">
      <h2 id="games-heading">{GAMES_HEADING}</h2>
      <p className="panel-copy">{GAMES_INTRO}</p>

      {status === 'idle' || status === 'loading' ? (
        <p className="status-message" role="status">
          Loading games…
        </p>
      ) : null}

      {status === 'error' ? (
        <p className="status-message status-message-error" role="status">
          {error}
        </p>
      ) : null}

      {status === 'ready' && games.length === 0 ? (
        <p className="status-message" role="status">
          No games available yet.
        </p>
      ) : null}

      {hasCatalog ? (
        <div className="mb-4">
          <label className="sr-only" htmlFor="game-search">
            {GAMES_SEARCH_LABEL}
          </label>
          <Input
            id="game-search"
            type="search"
            className="rounded-full"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={GAMES_SEARCH_PLACEHOLDER}
            autoComplete="off"
          />
        </div>
      ) : null}

      {hasCatalog && visibleGames.length === 0 ? (
        <div className="py-8 text-center" role="status">
          <p className="m-0 font-semibold text-foreground">{GAMES_SEARCH_EMPTY}</p>
          <p className="mt-1 mb-0 text-sm text-muted-foreground">{GAMES_SEARCH_EMPTY_HINT}</p>
        </div>
      ) : null}

      {hasCatalog && visibleGames.length > 0 ? (
        <ul className="game-list">
          {visibleGames.map((game) => (
            <GameCard key={game.id} game={game} />
          ))}
        </ul>
      ) : null}
    </section>
  )
}
