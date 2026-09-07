import { displayName } from '../../lib/tables'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { Card, CardContent } from '../ui/card'

/** Room members who hold no seat yet — the room's member list, in the group's words. */
export default function GroupSpectatorList({ players, userId }) {
  return (
    <section className="px-4 pt-4" aria-label="Picking a seat">
      <Card className="py-4">
        <CardContent className="flex flex-col gap-3 px-4">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Picking a seat
          </p>
          {players.length === 0 ? (
            <p className="text-sm text-muted-foreground">Everyone here has a seat.</p>
          ) : (
            <ul className="flex flex-col gap-2">
              {players.map((player) => (
                <li key={player.id} className="flex items-center gap-2 text-sm">
                  <PlayerAvatar user={player} size="sm" />
                  <span>{player.id === userId ? 'You' : displayName(player)}</span>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </section>
  )
}
