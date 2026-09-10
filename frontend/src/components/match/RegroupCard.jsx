import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { displayName } from '../../lib/tables'
import { cn } from '../../lib/utils'
import {
  formatBackToGame,
  formatRegroupInCount,
  REGROUP_ANOTHER_ROUND,
  REGROUP_BACK_TO_TABLE,
  REGROUP_CHOOSE_AGAIN,
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
 * Only `IN` counts. `PENDING` means the player has not returned or has not chosen yet, and
 * the table's seat count is no substitute either, in both directions. A group whose mode has
 * nothing to choose is re-seated the moment their match completes, so seats read as full
 * before anyone has opted in; a group whose mode has a role or an option to pick is left
 * standing on purpose, so an opted-in player can hold no seat at all (JQ-232).
 *
 * This number is reported, never enforced. `playAgain` is the only thing in the whole
 * system that moves a participant to IN, and this card's primary button is its only
 * caller — so gating that button on the count deadlocked the feature: two players both
 * landing here PENDING would each wait forever for the other to become IN. Whether the
 * next match may actually start is settled at the table, by `canStart` and the king.
 */
function countIn(participants) {
  return participants.filter((participant) => participant?.regroup === 'IN').length
}

/** The viewer's own answer, or null when they are not on this roster at all. */
function viewerRegroup(participants, viewerId) {
  if (!viewerId) {
    return null
  }
  return participants.find((participant) => participant?.user?.id === viewerId)?.regroup ?? null
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
 * The regroup roster and the ways out of it: another round with this group, back to the
 * game — or, for a solo player whose selections would otherwise be replayed, back to the
 * game to make them again — and away entirely.
 */
export default function RegroupCard({
  result,
  viewerId,
  minPlayers,
  busy = false,
  onPlayAgain,
  onDecline,
  onBackToGame,
  onChooseAgain,
}) {
  const participants = result?.participants ?? []
  const game = result?.game
  const inCount = countIn(participants)
  // Already IN means the seat is claimed and the table exists: opting in again is not a
  // thing to ask for, so the same button becomes the way back to that table.
  const viewerIn = viewerRegroup(participants, viewerId) === 'IN'
  // A solo player's primary action replays the role and options they just had, so they are
  // the only ones who need a way to reach those choices again — a group was never given
  // them back (JQ-232).
  const showChooseAgain =
    result?.groupPlay === false && Boolean(result?.mode?.hasPreMatchChoice) && !viewerIn
  // Both lead back to the game, so only one is offered; the more specific promise wins. The
  // `modes.length > 1` gate on the general one is JQ-233's to revisit, not this change's.
  const showBackToGame = !showChooseAgain && (game?.modes?.length ?? 0) > 1

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
          {formatRegroupInCount(inCount, participants.length, minPlayers)}
        </p>
        <div className="flex flex-col gap-2">
          <Button type="button" disabled={busy} onClick={onPlayAgain}>
            {viewerIn ? REGROUP_BACK_TO_TABLE : REGROUP_ANOTHER_ROUND}
          </Button>
          {showChooseAgain ? (
            <Button type="button" variant="outline" disabled={busy} onClick={onChooseAgain}>
              {REGROUP_CHOOSE_AGAIN}
            </Button>
          ) : null}
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
