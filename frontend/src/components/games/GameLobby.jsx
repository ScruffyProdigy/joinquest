import { useEffect, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { fetchGames } from '../../lib/games'
import GameCard from './GameCard'
import { GAMES_HEADING, GAMES_INTRO } from '../../lib/playerCopy'

export default function GameLobby() {
  const { user, loading: authLoading } = useAuth()
  const [games, setGames] = useState([])
  const [status, setStatus] = useState('idle')
  const [error, setError] = useState('')

  useEffect(() => {
    if (authLoading || !user) {
      return
    }

    let cancelled = false
    setStatus('loading')
    setError('')

    fetchGames(user.id)
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
  }, [authLoading, user, user?.id])

  if (authLoading || !user) {
    return null
  }

  return (
    <section className="game-lobby panel-card" aria-labelledby="games-heading">
      <h2 id="games-heading">{GAMES_HEADING}</h2>
      <p className="panel-copy">{GAMES_INTRO}</p>

      {status === 'loading' ? (
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
