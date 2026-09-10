import { accentColorFor } from '../../lib/gameAccent'
import { groupStatusLine } from '../../lib/group'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { Button } from '../ui/button'

/**
 * Full width, flat along the bottom (JQ-251). The prototype's header is the top of the
 * screen rather than a card floating on it, so it runs edge to edge and the blocks
 * below it are what carry the gutter.
 */
export default function GroupHeader({ table, room, busy, onLeave }) {
  const accent = accentColorFor(table?.game?.slug, table?.game?.accentColor)

  return (
    <header className="flex flex-col gap-3 px-4 pb-6 pt-4" style={{ background: accent.headerBg }}>
      <div className="flex items-start justify-between gap-3">
        <Button
          type="button"
          variant="secondary"
          size="sm"
          aria-label="Leave group"
          disabled={busy}
          onClick={onLeave}
        >
          ←
        </Button>
        <div className="flex -space-x-2" aria-label="In this group">
          {(room?.members ?? []).map((member) => (
            <PlayerAvatar key={member.id} user={member} size="sm" />
          ))}
        </div>
      </div>
      <div>
        <h1 className="text-2xl font-semibold">Your Group</h1>
        <p className="text-sm opacity-70" role="status">
          {groupStatusLine(table)}
        </p>
      </div>
    </header>
  )
}
