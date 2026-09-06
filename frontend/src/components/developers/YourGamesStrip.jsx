import { useEffect, useState } from 'react'
import { fetchMyGames, developerDashboardPath, visibilityLabel } from '../../lib/developers'
import { navigateTo } from '../../lib/usePathname'

export default function YourGamesStrip() {
  const [games, setGames] = useState([])
  const [status, setStatus] = useState('idle')

  useEffect(() => {
    let cancelled = false
    setStatus('loading')
    fetchMyGames()
      .then((items) => {
        if (!cancelled) {
          setGames(items)
          setStatus('ready')
        }
      })
      .catch(() => {
        if (!cancelled) {
          setGames([])
          setStatus('error')
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (status !== 'ready' || games.length === 0) {
    return null
  }

  const content = (
    <>
      <h2 id="your-games-heading">Your games</h2>
      <ul className="your-games-list">
        {games.map((game) => (
          <li key={game.id}>
            <button
              type="button"
              className="your-games-list__item"
              onClick={() => navigateTo(developerDashboardPath(game.id))}
            >
              <span className="your-games-list__name">{game.name}</span>
              <span className="your-games-list__meta">{visibilityLabel(game.visibility)}</span>
            </button>
          </li>
        ))}
      </ul>
    </>
  )

  return (
    <section className="your-games-strip panel-card" aria-labelledby="your-games-heading">
      {content}
    </section>
  )
}
