import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import { Badge } from '../ui/badge'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { displayName } from '../../lib/tables'
import { arrivalPartyOf, showsRegroupRoster } from '../../lib/regroup'
import { cn } from '../../lib/utils'
import {
  formatRegroupInCount,
  REGROUP_IN,
  REGROUP_OUT,
  REGROUP_PENDING,
  REGROUP_TITLE,
  RESULTS_YOU,
} from '../../lib/playerCopy'

const STATE_LABEL = {
  IN: REGROUP_IN,
  OUT: REGROUP_OUT,
  PENDING: REGROUP_PENDING,
}

/**
 * Only `IN` counts. `PENDING` means the player has not returned or has not chosen yet, and
 * the table's seat count is no substitute either, in both directions. A group whose mode has
 * nothing to choose is re-seated the moment their match completes, so seats read as full
 * before anyone has opted in; a group whose mode has a role or an option to pick is left
 * standing on purpose, so an opted-in player can hold no seat at all (JQ-232).
 *
 * This number is reported, never enforced — see `regroupActionsFor`, which is where the
 * refusal to gate the primary action on it now lives.
 */
function countIn(participants) {
  return participants.filter((participant) => participant?.regroup === 'IN').length
}

function RegroupRow({ participant, viewerId }) {
  const { user, role, regroup } = participant
  const isViewer = Boolean(viewerId) && user?.id === viewerId
  const name = isViewer ? RESULTS_YOU : displayName(user)
  const label = STATE_LABEL[regroup] ?? REGROUP_PENDING

  return (
    <li
      className={cn(
        'flex items-center gap-3 rounded-lg border px-3 py-2',
        isViewer ? 'border-primary/60 bg-primary/10' : 'border-border/60 bg-background/30',
      )}
    >
      <PlayerAvatar user={user} size="sm" />
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm text-foreground">{name}</span>
        {role ? <span className="truncate text-xs text-muted-foreground">{role}</span> : null}
      </div>
      <Badge variant={regroup === 'IN' ? 'success' : 'secondary'}>{label}</Badge>
    </li>
  )
}

/**
 * Who is coming back, and how far off a table is.
 *
 * The roster and nothing else — the ways out of the match live in the screen's action bar,
 * where the prototype puts them and where they can still be offered to the players this card
 * is not for (JQ-277). It renders for exactly one of them: a group who played the match out.
 * See `showsRegroupRoster` for why solo and exited-early are not on that list.
 *
 * The viewer's own group, not the match's standings. A 3v3 is ordinarily two groups who each
 * queued from their own room, and listing all six interleaved by role — with nothing marking
 * which three were yours — was the card asking you to regroup with strangers you never
 * agreed to come back with (JQ-291). The count below it reads the same way: "2 of 3 back and
 * in" is about your group, and the mode minimum beside it is what backfill still has to fill.
 */
export default function RegroupCard({ result, viewerId, minPlayers }) {
  if (!showsRegroupRoster(result, viewerId)) {
    return null
  }
  const participants = arrivalPartyOf(result)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{REGROUP_TITLE}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <ul className="flex flex-col gap-2">
          {participants.map((participant) => (
            <RegroupRow
              key={participant.user?.id ?? participant.user?.displayName}
              participant={participant}
              viewerId={viewerId}
            />
          ))}
        </ul>
        <p className="text-xs text-muted-foreground" data-testid="regroup-count">
          {formatRegroupInCount(countIn(participants), participants.length, minPlayers)}
        </p>
      </CardContent>
    </Card>
  )
}
