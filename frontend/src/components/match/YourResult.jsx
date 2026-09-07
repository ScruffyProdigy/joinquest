import { Card, CardContent } from '../ui/card'
import { Badge } from '../ui/badge'
import { matchHeadline, RESULTS_YOUR_RESULT_SO_FAR } from '../../lib/playerCopy'

/**
 * The viewer's own outcome, for the still-playing screen. Without it that branch shows a
 * player everything except the thing they came back to see: how they did. StillPlaying
 * lists only the others, by design.
 *
 * Renders nothing when the viewer is not in the roster — an unauthenticated or non-
 * participant read has no "your result" to show, and an empty card would be worse than
 * no card.
 */
export default function YourResult({ result, viewerId }) {
  const participants = result?.participants ?? []
  const viewer = participants.find((participant) => participant.user?.id === viewerId)
  if (!viewer) {
    return null
  }

  const { headline, sub } = matchHeadline({
    reason: viewer.reason,
    complete: Boolean(result?.complete),
    placement: viewer.placement,
    playerCount: participants.length,
  })

  return (
    <Card>
      <CardContent className="flex flex-col gap-2">
        <Badge variant="secondary">{RESULTS_YOUR_RESULT_SO_FAR}</Badge>
        <p className="font-heading text-2xl font-bold text-foreground">{headline}</p>
        <p className="text-sm text-muted-foreground">{sub}</p>
      </CardContent>
    </Card>
  )
}
