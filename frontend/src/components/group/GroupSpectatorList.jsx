import { useEffect, useRef, useState } from 'react'
import { displayName } from '../../lib/tables'
import { cn } from '../../lib/utils'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { Card, CardContent } from '../ui/card'

/** How long a decline stays on the card before the row goes, and when it starts fading. */
const OUT_BEAT_MS = 3000
const OUT_FADE_MS = 2700

function ClockIcon() {
  return (
    <svg viewBox="0 0 24 24" className="size-3" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

/**
 * Watch for a previous-match player flipping to OUT while this card is on screen, and
 * hold their row for a beat so the decline reads as a decision rather than a
 * disappearance.
 *
 * Only a flip counts. A player who was already OUT when the page loaded is history, not
 * news: announcing it would report a decision the viewer never saw being made.
 */
function useDeclineBeat(players) {
  const seenRef = useRef(null)
  const timersRef = useRef(new Map())
  const [declining, setDeclining] = useState([])

  useEffect(() => {
    const previous = seenRef.current
    seenRef.current = new Map(players.map((entry) => [entry.user?.id, entry.status]))
    if (!previous) {
      return
    }
    for (const entry of players) {
      const id = entry.user?.id
      if (!id || entry.status !== 'out' || timersRef.current.has(id)) {
        continue
      }
      if (!previous.has(id) || previous.get(id) === 'out') {
        continue
      }
      setDeclining((prev) => [...prev, { user: entry.user, leaving: false }])
      timersRef.current.set(id, [
        setTimeout(
          () =>
            setDeclining((prev) =>
              prev.map((row) => (row.user?.id === id ? { ...row, leaving: true } : row)),
            ),
          OUT_FADE_MS,
        ),
        setTimeout(() => {
          setDeclining((prev) => prev.filter((row) => row.user?.id !== id))
          timersRef.current.delete(id)
        }, OUT_BEAT_MS),
      ])
    }
  }, [players])

  const timers = timersRef.current
  useEffect(
    () => () => {
      for (const pair of timers.values()) {
        pair.forEach(clearTimeout)
      }
      timers.clear()
    },
    [timers],
  )

  return declining
}

function Row({ user, userId, status, leaving }) {
  return (
    <li
      className={cn(
        'flex items-center gap-2 text-sm',
        status === 'awaiting' && 'opacity-45',
        status === 'out' && (leaving ? 'animate-out fade-out' : 'animate-in fade-in'),
      )}
    >
      <PlayerAvatar user={user} size="sm" />
      <span className="flex-1 truncate">{user?.id === userId ? 'You' : displayName(user)}</span>
      {status === 'awaiting' ? (
        <span className="flex items-center gap-1 text-xs text-muted-foreground">
          <ClockIcon />
          Awaiting
        </span>
      ) : null}
      {status === 'out' ? <span className="text-xs text-muted-foreground">Out</span> : null}
    </li>
  )
}

/**
 * Everyone still to decide: room members who hold no seat, and the previous match's
 * players who have not answered. One list, as the prototype has it — the Awaiting badge
 * is what separates the two populations, not a second heading.
 *
 * The card is absent, not empty, when the viewer is the only person here. A card whose
 * whole content is your own name says nothing.
 */
export default function GroupSpectatorList({ players = [], userId }) {
  const declining = useDeclineBeat(players)

  const rows = [
    ...players.filter((entry) => entry.status !== 'out'),
    ...declining.map((row) => ({ user: row.user, status: 'out', leaving: row.leaving })),
  ]
  if (!rows.some((row) => row.user?.id !== userId)) {
    return null
  }

  return (
    <section className="px-4 pt-4" aria-label="Picking a seat">
      <Card className="py-4">
        <CardContent className="flex flex-col gap-3 px-4">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Picking a seat
          </p>
          <ul className="flex flex-col gap-2">
            {rows.map((row) => (
              <Row
                key={row.user?.id}
                user={row.user}
                userId={userId}
                status={row.status}
                leaving={row.leaving}
              />
            ))}
          </ul>
        </CardContent>
      </Card>
    </section>
  )
}
