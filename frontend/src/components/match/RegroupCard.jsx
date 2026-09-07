import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { displayName } from '../../lib/tables'
import { cn } from '../../lib/utils'
import {
  formatBackToGame,
  formatNeedMorePlayers,
  REGROUP_ANOTHER_ROUND,
  REGROUP_FIND_SOMETHING_NEW,
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
 * Only `IN` counts toward the gate. `PENDING` means the player has not returned or has
 * not chosen yet, and counting them would let the group start a match short-handed —
 * the exact failure the three-state roster exists to prevent. The table's seat count is
 * no substitute either: a room-table group is re-seated automatically when their match
 * completes, so seats read as full before anyone has actually opted in.
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
      <Badge variant={regroup === 'IN' ? 'default' : 'secondary'}>{label}</Badge>
    </li>
  )
}

/**
 * The regroup roster and the three ways out of it: another round with this group,
 * back to the game's other modes, or away entirely.
 */
export default function RegroupCard({
  result,
  viewerId,
  minPlayers,
  busy = false,
  onPlayAgain,
  onDecline,
  onBackToGame,
}) {
  const participants = result?.participants ?? []
  const game = result?.game
  const inCount = countIn(participants)
  const required = Number.isFinite(minPlayers) ? minPlayers : 0
  const missing = required - inCount
  const ready = missing <= 0
  const showBackToGame = (game?.modes?.length ?? 0) > 1

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
        <div className="flex flex-col gap-2">
          <Button type="button" disabled={!ready || busy} onClick={onPlayAgain}>
            {ready ? REGROUP_ANOTHER_ROUND : formatNeedMorePlayers(missing)}
          </Button>
          {showBackToGame ? (
            <Button type="button" variant="outline" disabled={busy} onClick={onBackToGame}>
              {formatBackToGame(game?.name)}
            </Button>
          ) : null}
          <Button type="button" variant="ghost" disabled={busy} onClick={onDecline}>
            {REGROUP_FIND_SOMETHING_NEW}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
