import { useEffect, useRef, useState } from 'react'
import { ClockIcon, MoonIcon } from 'lucide-react'
import { displayName } from '../../lib/tables'
import { cn } from '../../lib/utils'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { Card, CardContent } from '../ui/card'

/** How long a decline stays on the card before the row goes, and when it starts fading. */
const OUT_BEAT_MS = 3000
const OUT_FADE_MS = 2700

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

/**
 * `away` is deliberately independent of `status` (JQ-265): a player can be awaiting a
 * regroup answer and away at the same time, and the card has to be able to say both.
 *
 * It earns a caption of its own rather than only dimming the avatar. Opacity alone is a
 * weak carrier next to the Awaiting badge — a faded avatar beside a full-strength name
 * reads as a rendering artefact, not a statement — and the two states sit in the same list,
 * so the one without words loses every comparison. The still moon against Awaiting's
 * turning clock is the distinction: that player is being waited ON, this one has gone
 * quiet.
 *
 * The name itself stays at full strength either way. The doubt is about whether they are
 * watching, never about who they are.
 */
function Row({ user, userId, status, leaving, away = false }) {
  return (
    <li
      className={cn(
        'flex items-center gap-3 py-2',
        // The prototype slides each arrival up into place; rows are keyed by player, so
        // only a genuinely new one plays it.
        'motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-1',
        status === 'awaiting' && 'opacity-45',
        status === 'out' && (leaving ? 'animate-out fade-out' : 'animate-in fade-in'),
      )}
    >
      <PlayerAvatar user={user} size="xs" away={away} />
      <span className="flex-1 truncate text-sm font-bold text-foreground">
        {user?.id === userId ? 'You' : displayName(user)}
      </span>
      {/*
        Badges share one shrink-0 row so the name is what truncates when a player carries
        both. Away comes first because it qualifies the other: "they have not answered, and
        we cannot see them" is the order those facts are useful in.
      */}
      <span className="flex shrink-0 items-center gap-2">
        {away ? (
          <span className="flex items-center gap-1 text-2xs font-normal text-muted-foreground">
            <MoonIcon size={11} aria-hidden="true" />
            Away
          </span>
        ) : null}
        {status === 'awaiting' ? (
          <span className="flex items-center gap-1 text-2xs font-normal text-muted-foreground">
            {/* The prototype turns the hand once every three seconds, linear and forever. */}
            <ClockIcon
              size={11}
              aria-hidden="true"
              className="motion-safe:animate-spin [animation-duration:3s]"
            />
            Awaiting
          </span>
        ) : null}
        {status === 'out' ? (
          <span className="text-2xs font-normal text-muted-foreground">Out</span>
        ) : null}
      </span>
    </li>
  )
}

/**
 * Everyone still to decide: room members who hold no seat, and the previous match's
 * players who have not answered. One list, as the prototype has it — the Awaiting badge
 * is what separates the two populations, not a second heading.
 *
 * The viewer counts. The prototype seeds this list with `You` and only takes the row away
 * once the seat is claimed, so the card is the standing answer to "who still has to sit
 * down", and on the common path — you open a fresh group and look at it before claiming —
 * you are the whole list. Production used to hide that case as saying nothing, which in
 * practice meant most players never saw this card at all.
 *
 * Nobody left to list at all — everyone seated — takes the card away entirely.
 */
export default function GroupSpectatorList({ players = [], userId }) {
  const declining = useDeclineBeat(players)

  const rows = [
    ...players.filter((entry) => entry.status !== 'out'),
    ...declining.map((row) => ({ user: row.user, status: 'out', leaving: row.leaving })),
  ]
  if (rows.length === 0) {
    return null
  }

  return (
    <section className="px-4 pt-4" aria-label="Picking a seat">
      <Card className="gap-0 py-0">
        <CardContent className="px-4 pt-4 pb-3">
          <p className="mb-3 text-2xs font-bold uppercase tracking-widest text-muted-foreground">
            Picking a seat
          </p>
          {/*
            Tailwind loads here without preflight (see tailwind.css), so a `ul` that does
            not opt out keeps the browser's disc marker and 40px indent — which is what
            pushed these rows off the card's own left edge.
          */}
          <ul className="m-0 list-none space-y-1 p-0" role="list">
            {rows.map((row) => (
              <Row
                key={row.user?.id}
                user={row.user}
                userId={userId}
                status={row.status}
                leaving={row.leaving}
                away={row.away}
              />
            ))}
          </ul>
        </CardContent>
      </Card>
    </section>
  )
}
