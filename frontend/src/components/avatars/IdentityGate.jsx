import { useState } from 'react'
import { notifyAuthComplete } from '../../lib/authBroadcast'
import {
  AVATAR_PROMPT_HEADING,
  AVATAR_PROMPT_TAGLINE,
  IDENTITY_GATE_BACK,
  IDENTITY_GATE_BACK_TO_AVATARS,
  IDENTITY_GATE_DIVIDER,
  IDENTITY_GATE_HEADING,
  IDENTITY_GATE_SIGN_IN,
  IDENTITY_GATE_TAGLINE,
  NAME_PROMPT_HEADING,
  NAME_PROMPT_TAGLINE,
} from '../../lib/playerCopy'
import {
  IDENTITY_GAP_AVATAR,
  IDENTITY_GAP_NAME,
  VIEWER_MEMBER,
  identityGap,
  viewerTier,
} from '../../lib/viewer'
import { Button } from '../ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../ui/dialog'
import { useAuth } from '../auth/AuthProvider'
import SignInPanel from '../auth/SignInPanel'
import AvatarPrompt from './AvatarPrompt'
import DisplayNamePrompt from './DisplayNamePrompt'
import GuestIdentityPicker from './GuestIdentityPicker'

/** Heading, tagline, and the label to get back from sign-in, per missing half. */
const FRAMING = {
  [IDENTITY_GAP_NAME]: {
    heading: NAME_PROMPT_HEADING,
    tagline: NAME_PROMPT_TAGLINE,
    back: IDENTITY_GATE_BACK,
  },
  [IDENTITY_GAP_AVATAR]: {
    heading: AVATAR_PROMPT_HEADING,
    tagline: AVATAR_PROMPT_TAGLINE,
    back: IDENTITY_GATE_BACK_TO_AVATARS,
  },
  both: {
    heading: IDENTITY_GATE_HEADING,
    tagline: IDENTITY_GATE_TAGLINE,
    back: IDENTITY_GATE_BACK_TO_AVATARS,
  },
}

/**
 * The gate asks for whatever is missing and no more: both halves from a
 * visitor, a name alone from an account whose name was never chosen, a face
 * alone from one that has a name. There is no way out except finishing — no
 * close button, no outside click, no escape key.
 *
 * The sign-in path is offered by *tier*, not by what is missing: it is a real
 * upgrade for a guest, and nonsense to show someone already holding an account.
 */
export default function IdentityGate() {
  const { user, loading, acceptSessionUser } = useAuth()
  const [showSignIn, setShowSignIn] = useState(false)
  const [busy, setBusy] = useState(false)

  const gap = identityGap(user)
  if (loading || !gap) {
    return null
  }

  const framing = FRAMING[gap]
  const offerSignIn = viewerTier(user) !== VIEWER_MEMBER

  function handleSaved(updated) {
    acceptSessionUser(updated)
    notifyAuthComplete()
  }

  return (
    <Dialog open>
      <DialogContent
        showCloseButton={false}
        className="max-w-xl gap-6 border-border/60 bg-background/95 backdrop-blur"
        onEscapeKeyDown={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <div className="flex flex-col items-center gap-1 text-center">
          <DialogTitle className="font-heading text-2xl font-semibold">{framing.heading}</DialogTitle>
          <DialogDescription>{framing.tagline}</DialogDescription>
        </div>

        {showSignIn ? (
          <div className="flex flex-col gap-4">
            <SignInPanel heading={null} showGuestOption={false} />
            <Button type="button" variant="link" onClick={() => setShowSignIn(false)}>
              {framing.back}
            </Button>
          </div>
        ) : (
          <>
            {gap === IDENTITY_GAP_NAME ? (
              <DisplayNamePrompt user={user} onSaved={handleSaved} onBusyChange={setBusy} />
            ) : null}
            {gap === IDENTITY_GAP_AVATAR ? (
              <AvatarPrompt user={user} onSaved={handleSaved} onBusyChange={setBusy} />
            ) : null}
            {gap !== IDENTITY_GAP_NAME && gap !== IDENTITY_GAP_AVATAR ? (
              <GuestIdentityPicker user={user} onSaved={handleSaved} onBusyChange={setBusy} />
            ) : null}

            {offerSignIn ? (
              <div className="flex flex-col items-center gap-2">
                <span className="text-2xs uppercase tracking-wide text-muted-foreground">
                  {IDENTITY_GATE_DIVIDER}
                </span>
                <Button
                  type="button"
                  variant="link"
                  className="text-sm"
                  disabled={busy}
                  onClick={() => setShowSignIn(true)}
                >
                  {IDENTITY_GATE_SIGN_IN}
                </Button>
              </div>
            ) : null}
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
