import { Card, CardContent } from '../ui/card'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { displayName } from '../../lib/tables'
import { RESULTS_IN_PROGRESS } from '../../lib/playerCopy'

/** The other players, still shown as in progress, while the viewer waits. */
export default function StillPlaying({ participants }) {
  const list = participants ?? []

  return (
    <Card>
      <CardContent>
        <ul className="flex flex-col gap-2">
          {list.map((participant) => (
            <li
              key={participant.user?.id ?? participant.user?.displayName}
              className="flex items-center gap-3 rounded-lg border border-border/60 bg-background/30 px-3 py-2"
            >
              <PlayerAvatar user={participant.user} size="sm" />
              <div className="flex min-w-0 flex-1 flex-col">
                <span className="truncate text-sm text-foreground">{displayName(participant.user)}</span>
                {participant.role ? (
                  <span className="truncate text-xs text-muted-foreground">{participant.role}</span>
                ) : null}
              </div>
              <span className="text-xs text-muted-foreground">{RESULTS_IN_PROGRESS}</span>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}
