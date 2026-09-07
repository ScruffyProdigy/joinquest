import { Card, CardHeader, CardTitle, CardContent } from '../ui/card'
import { Badge } from '../ui/badge'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { displayName } from '../../lib/tables'
import { cn } from '../../lib/utils'
import {
  ordinal,
  RESULTS_FINAL_TITLE,
  RESULTS_IN_PROGRESS_TITLE,
  RESULTS_PLACEMENT_UNKNOWN,
  RESULTS_STILL_PLAYING,
  RESULTS_WINNER,
  RESULTS_YOU,
} from '../../lib/playerCopy'

/**
 * Known placements sort first (ascending), then everyone still unplaced —
 * stable within each group so ties/unknowns keep the order the server sent.
 */
function sortParticipants(participants, reported) {
  return participants
    .map((participant, index) => ({ participant, index }))
    .sort((a, b) => {
      const placementA = reported ? a.participant.placement : null
      const placementB = reported ? b.participant.placement : null
      const knownA = placementA != null
      const knownB = placementB != null
      if (knownA && knownB) return placementA - placementB
      if (knownA) return -1
      if (knownB) return 1
      return a.index - b.index
    })
    .map((entry) => entry.participant)
}

function StandingRow({ participant, viewerId, reported }) {
  const { user, role, finished, winner, placement } = participant
  const isViewer = Boolean(viewerId) && user?.id === viewerId
  const name = isViewer ? RESULTS_YOU : displayName(user)
  const knownPlacement = reported && placement != null
  const isWinner = reported && Boolean(winner)

  return (
    <li
      className={cn(
        'flex items-center gap-3 rounded-lg border px-3 py-2',
        isViewer ? 'border-primary/60 bg-primary/10' : 'border-border/60 bg-background/30',
      )}
    >
      <span className="flex w-6 shrink-0 items-center justify-center text-sm text-muted-foreground">
        {knownPlacement ? (
          ordinal(placement)
        ) : (
          <>
            <span className="size-2 animate-pulse rounded-full bg-muted-foreground" aria-hidden="true" />
            {/* The "Still playing" badge already announces this for an unfinished row — only add
                text here for the case that badge doesn't cover: reported-finished-but-unplaced. */}
            {finished ? <span className="sr-only">{RESULTS_PLACEMENT_UNKNOWN}</span> : null}
          </>
        )}
      </span>
      <PlayerAvatar user={user} size="sm" />
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm text-foreground">{name}</span>
        {role ? <span className="truncate text-xs text-muted-foreground">{role}</span> : null}
      </div>
      {isWinner ? <Badge>{RESULTS_WINNER}</Badge> : null}
      {!finished ? <Badge variant="secondary">{RESULTS_STILL_PLAYING}</Badge> : null}
    </li>
  )
}

export default function MatchStandings({ result, viewerId }) {
  const complete = Boolean(result?.complete)
  const reported = result?.reported !== false
  const participants = sortParticipants(result?.participants ?? [], reported)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{complete ? RESULTS_FINAL_TITLE : RESULTS_IN_PROGRESS_TITLE}</CardTitle>
      </CardHeader>
      <CardContent>
        <ul className="flex flex-col gap-2">
          {participants.map((participant) => (
            <StandingRow
              key={participant.user?.id ?? participant.user?.displayName}
              participant={participant}
              viewerId={viewerId}
              reported={reported}
            />
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}
