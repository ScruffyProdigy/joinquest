import { useEffect, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { fetchGames } from '../../lib/games'
import GameCard from './GameCard'

export default function GameLobby({ headingId }) {
  const { user, loading: authLoading } = useAuth()
  const [games, setGames] = useState([])
  const [status, setStatus] = useState('idle')
  const [error, setError] = useState('')

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

  return (
    // The visible heading lives in the page header above, so point at it rather
    // than repeating it here.
    <section className="game-lobby" aria-labelledby={headingId}>
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

      {status === 'ready' && games.length > 0 ? (
        <ul className="game-list">
          {games.map((game) => (
            <GameCard key={game.id} game={game} />
          ))}
        </ul>
      ) : null}
    </section>
  )
}
