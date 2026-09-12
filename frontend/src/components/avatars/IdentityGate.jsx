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
import { Link } from '../ui/link'
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
 *
 * It is a takeover rather than a centred card, which is what the prototype
 * draws and what a phone needs: the card shape floated a screen's worth of
 * content over the page with nothing holding it in, so the bottom of it — the
 * sign-in offer, and the button that starts the game — fell off the end of
 * anything shorter than a tall handset, with the page behind showing through
 * where the gate had run out of room.
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

  /**
   * The prototype's rule, and the reason this is passed down rather than
   * rendered here: the sign-in offer belongs under the choices it is an
   * alternative to, inside the same block, not in one of its own.
   */
  const signIn = offerSignIn ? (
    <div className="mt-6">
      <div className="mb-6 flex items-center gap-3">
        <span className="h-px flex-1 bg-border" aria-hidden="true" />
        <span className="text-sm text-muted-foreground">{IDENTITY_GATE_DIVIDER}</span>
        <span className="h-px flex-1 bg-border" aria-hidden="true" />
      </div>
      <Link variant="pill" disabled={busy} onClick={() => setShowSignIn(true)}>
        {IDENTITY_GATE_SIGN_IN}
      </Link>
    </div>
  ) : null

  return (
    <Dialog open>
      <DialogContent
        takeover
        showCloseButton={false}
        onEscapeKeyDown={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <div className="w-full shrink-0 pb-2">
          {/* 30px rather than the scale's 34, and rather than the prototype's 32:
              our heading face is wider than the prototype's, and 30 is where
              "Welcome to JoinQuest" still lands on one line at 375px. The
              prototype's composition is a single-line greeting, so the line
              matters more here than the number does. */}
          <DialogTitle className="w-full text-center font-heading text-[1.875rem] leading-tight font-bold">
            {framing.heading}
          </DialogTitle>
          <DialogDescription className="w-full text-center font-heading text-lg font-normal">
            {framing.tagline}
          </DialogDescription>
        </div>

        {showSignIn ? (
          <div className="flex w-full flex-1 flex-col justify-center gap-4 pb-8">
            <SignInPanel heading={null} showGuestOption={false} />
            <Link variant="quiet" onClick={() => setShowSignIn(false)}>
              {framing.back}
            </Link>
          </div>
        ) : (
          <>
            {gap === IDENTITY_GAP_NAME ? (
              <div className="flex w-full flex-1 flex-col justify-center pb-8">
                <div className="flex flex-col gap-6">
                  <DisplayNamePrompt user={user} onSaved={handleSaved} onBusyChange={setBusy} />
                </div>
                {signIn}
              </div>
            ) : null}
            {gap === IDENTITY_GAP_AVATAR ? (
              <div className="flex w-full flex-1 flex-col justify-center pb-8">
                <div className="flex flex-col gap-6">
                  <AvatarPrompt user={user} onSaved={handleSaved} onBusyChange={setBusy} />
                </div>
                {signIn}
              </div>
            ) : null}
            {gap !== IDENTITY_GAP_NAME && gap !== IDENTITY_GAP_AVATAR ? (
              <GuestIdentityPicker
                user={user}
                onSaved={handleSaved}
                onBusyChange={setBusy}
                signIn={signIn}
              />
            ) : null}
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
