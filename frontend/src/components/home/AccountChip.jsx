import { useState } from 'react'
import { ACCOUNT_CHIP_FALLBACK, SIGN_IN_OR_JOIN } from '../../lib/playerCopy'
import { chosenDisplayName } from '../../lib/viewer'
import { Button } from '../ui/button'
import { useAuth } from '../auth/AuthProvider'
import PlayerAvatar from '../avatars/PlayerAvatar'
import SignInDialog from '../auth/SignInDialog'

/**
 * The one auth control on home. A guest gets the sign-in pill; someone with a
 * real account gets their avatar, which is how they reach account settings.
 */
export default function AccountChip() {
  const { user, loading } = useAuth()
  const [signInOpen, setSignInOpen] = useState(false)

  // Nothing until the session is known, so the chip does not flip label on load.
  if (loading) {
    return null
  }

  if (!user || user.isGuest) {
    return (
      <>
        <Button type="button" className="shrink-0 rounded-full" onClick={() => setSignInOpen(true)}>
          {SIGN_IN_OR_JOIN}
        </Button>
        <SignInDialog open={signInOpen} onOpenChange={setSignInOpen} />
      </>
    )
  }

  return (
    <Button variant="ghost" className="h-auto shrink-0 gap-2 rounded-full py-1 pr-3 pl-1" asChild>
      {/* A raw <a> on purpose (JQ-72): it wraps the avatar and name, not text, so
          link styling would have nothing to colour. */}
      <a href="/account">
        <PlayerAvatar user={user} size="sm" />
        {/* Without a fallback the link would have no accessible name at all
            for someone who signed in but has not picked a name yet. */}
        <span className="max-w-32 truncate">{chosenDisplayName(user) || ACCOUNT_CHIP_FALLBACK}</span>
      </a>
    </Button>
  )
}
