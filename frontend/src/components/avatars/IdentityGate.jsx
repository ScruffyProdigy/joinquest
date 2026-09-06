import { useState } from 'react'
import { createGuestSession } from '../../lib/auth'
import { notifyAuthComplete } from '../../lib/authBroadcast'
import { updatePlayerProfile } from '../../lib/avatars'
import { generateGuestIdentities } from '../../lib/guestIdentity'
import {
  IDENTITY_GATE_DIVIDER,
  IDENTITY_GATE_ERROR,
  IDENTITY_GATE_HEADING,
  IDENTITY_GATE_PROMPT,
  IDENTITY_GATE_SCOPE_HINT,
  IDENTITY_GATE_SIGN_IN,
  IDENTITY_GATE_TAGLINE,
} from '../../lib/playerCopy'
import { needsIdentity } from '../../lib/viewer'
import { Button } from '../ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../ui/dialog'
import { useAuth } from '../auth/AuthProvider'
import SignInPanel from '../auth/SignInPanel'

/**
 * First-entry gate: every visitor needs a name and an avatar before going further.
 * There is no way out except picking an avatar or signing in — no close button, no
 * outside click, no escape key.
 */
export default function IdentityGate() {
  const { user, loading, acceptSessionUser } = useAuth()
  const [identities] = useState(() => generateGuestIdentities())
  const [pendingName, setPendingName] = useState('')
  const [error, setError] = useState('')
  const [showSignIn, setShowSignIn] = useState(false)

  if (loading || !needsIdentity(user)) {
    return null
  }

  async function handlePick(identity) {
    if (pendingName) {
      return
    }
    setPendingName(identity.name)
    setError('')
    try {
      // A visitor with no session at all needs one before the profile can be saved.
      if (!user) {
        await createGuestSession()
      }
      const updated = await updatePlayerProfile(identity.name, identity.avatarKey)
      acceptSessionUser(updated)
      notifyAuthComplete()
    } catch (err) {
      setError(err.message || IDENTITY_GATE_ERROR)
    } finally {
      setPendingName('')
    }
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
          <DialogTitle className="font-heading text-2xl font-semibold">{IDENTITY_GATE_HEADING}</DialogTitle>
          <DialogDescription>{IDENTITY_GATE_TAGLINE}</DialogDescription>
        </div>

        {showSignIn ? (
          <div className="flex flex-col gap-4">
            <SignInPanel heading={null} showGuestOption={false} />
            <Button type="button" variant="link" onClick={() => setShowSignIn(false)}>
              Back to avatars
            </Button>
          </div>
        ) : (
          <>
            <div className="flex flex-col items-center gap-1 text-center">
              <p className="text-sm font-medium text-foreground">{IDENTITY_GATE_PROMPT}</p>
              <p className="text-2xs text-muted-foreground">{IDENTITY_GATE_SCOPE_HINT}</p>
            </div>

            <ul className="grid gap-2 sm:grid-cols-2" role="list">
              {identities.map((identity) => (
                <li key={identity.name}>
                  <button
                    type="button"
                    className="flex w-full items-center gap-3 rounded-full border border-border bg-muted/40 px-4 py-2.5 text-left transition-colors hover:bg-muted disabled:opacity-60"
                    disabled={Boolean(pendingName)}
                    onClick={() => void handlePick(identity)}
                  >
                    <img src={identity.imageUrl} alt="" className="size-8 shrink-0 rounded-full" />
                    <span className="font-mono-display text-sm text-foreground">
                      {pendingName === identity.name ? 'Starting…' : identity.name}
                    </span>
                  </button>
                </li>
              ))}
            </ul>

            {error ? (
              <p className="status-message status-message-error" role="status">
                {error}
              </p>
            ) : null}

            <div className="flex flex-col items-center gap-2">
              <span className="text-2xs uppercase tracking-wide text-muted-foreground">
                {IDENTITY_GATE_DIVIDER}
              </span>
              <Button
                type="button"
                variant="link"
                className="text-sm"
                disabled={Boolean(pendingName)}
                onClick={() => setShowSignIn(true)}
              >
                {IDENTITY_GATE_SIGN_IN}
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
