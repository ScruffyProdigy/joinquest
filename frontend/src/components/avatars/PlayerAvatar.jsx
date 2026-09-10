import { cn } from '../../lib/utils'
import { avatarInitial, resolveUserAvatarUrl } from '../../lib/avatars'
import { KING_LABEL } from '../../lib/playerCopy'
import { displayName } from '../../lib/tables'
import { Avatar, AvatarFallback, AvatarImage } from '../ui/avatar'

const SIZE_MAP = { xs: 'xs', sm: 'sm', md: 'md' }

export default function PlayerAvatar({ user, size = 'md', className = '', title, ring }) {
  const baseLabel = title ?? displayName(user)
  const label = ring === 'king' ? `${baseLabel} (${KING_LABEL})` : baseLabel
  const avatarUrl = resolveUserAvatarUrl(user)

  const avatar = (
    <Avatar
      size={SIZE_MAP[size] ?? 'md'}
      className={cn(ring === 'king' && 'ring-2 ring-primary ring-offset-2 ring-offset-background', className)}
      title={label}
      data-ring={ring === 'king' ? 'king' : undefined}
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
