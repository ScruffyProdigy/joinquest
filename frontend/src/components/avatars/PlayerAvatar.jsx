import { cn } from '../../lib/utils'
import { avatarInitial, resolveUserAvatarUrl } from '../../lib/avatars'
import { KING_LABEL } from '../../lib/playerCopy'
import { displayName } from '../../lib/tables'
import { Avatar, AvatarFallback, AvatarImage } from '../ui/avatar'

const SIZE_MAP = { xs: 'xs', sm: 'sm', md: 'md' }

/**
 * `away` is the roster's honesty about a member whose socket has been gone longer than
 * DefaultRoomMemberAwayGrace (JQ-265). It lives here rather than at each call site so the
 * three places that draw a roster dim it the same way and say the same word.
 *
 * Dimmed, not removed or badged: the player still holds their place, and the roster is
 * reporting reduced confidence rather than an event. The title carries the word too,
 * because opacity is not available to a screen reader and is not a safe carrier of meaning
 * on its own — same reason `ring === 'king'` names the king in the label instead of relying
 * on the ring.
 */
export default function PlayerAvatar({ user, size = 'md', className = '', title, ring, away = false }) {
  const baseLabel = title ?? displayName(user)
  const ringLabel = ring === 'king' ? `${baseLabel} (${KING_LABEL})` : baseLabel
  const label = away ? `${ringLabel} (away)` : ringLabel
  const avatarUrl = resolveUserAvatarUrl(user)

  const avatar = (
    <Avatar
      size={SIZE_MAP[size] ?? 'md'}
      className={cn(
        ring === 'king' && 'ring-2 ring-primary ring-offset-2 ring-offset-background',
        away && 'opacity-40 grayscale',
        className,
      )}
      title={label}
      data-ring={ring === 'king' ? 'king' : undefined}
      data-away={away ? 'true' : undefined}
    >
      {avatarUrl ? (
        <AvatarImage src={avatarUrl} alt="" aria-hidden="true" />
      ) : (
        <AvatarFallback aria-hidden="true">{avatarInitial(user)}</AvatarFallback>
      )}
    </Avatar>
  )

  return avatar
}
