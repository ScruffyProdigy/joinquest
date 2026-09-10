import { useEffect, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { fetchGames, filterGamesBySearch } from '../../lib/games'
import GameCard from './GameCard'
import DeveloperPromoCard from '../developers/DeveloperPromoCard'
import { Input } from '../ui/input'
import {
  ALL_GAMES_HEADING,
  GAMES_SEARCH_EMPTY,
  GAMES_SEARCH_EMPTY_HINT,
  GAMES_SEARCH_LABEL,
  GAMES_SEARCH_PLACEHOLDER,
} from '../../lib/playerCopy'

// A fixed slot keeps the promo in the same place whether the catalog holds three
// games or thirty. Clamped below, so a shorter catalog still ends with it.
const DEVELOPER_PROMO_SLOT = 3

/** The catalog labels itself, so anything that needs to point at it can. */
export const CATALOG_HEADING_ID = 'all-games-heading'

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
  // The promo rides along through a search. With nothing left it is all that remains,
  // and retitles itself rather than leaving the player at a dead end.
  const noResults = visibleGames.length === 0
  const promoSlot = Math.min(DEVELOPER_PROMO_SLOT, visibleGames.length)

  return (
    <section className="game-lobby" aria-labelledby={CATALOG_HEADING_ID}>
      {/* Small and uppercased by CSS, the way the prototype labels the grid. The
          constant stays sentence case so the accessible name is "All games". */}
      <h2 id={CATALOG_HEADING_ID} className="catalog-heading">
        {ALL_GAMES_HEADING}
      </h2>

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

      {hasCatalog && noResults ? (
        <div className="py-8 text-center" role="status">
          <p className="m-0 font-semibold text-foreground">{GAMES_SEARCH_EMPTY}</p>
          <p className="mt-1 mb-0 text-sm text-muted-foreground">{GAMES_SEARCH_EMPTY_HINT}</p>
        </div>
      ) : null}

      {hasCatalog ? (
        <ul className="game-list">
          {visibleGames.slice(0, promoSlot).map((game) => (
            <GameCard key={game.id} game={game} />
          ))}
          <DeveloperPromoCard noResults={noResults} />
          {visibleGames.slice(promoSlot).map((game) => (
            <GameCard key={game.id} game={game} />
          ))}
        </ul>
      ) : null}
    </section>
  )
}
