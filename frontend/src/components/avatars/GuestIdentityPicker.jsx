import { useState } from 'react'
import { createGuestSession } from '../../lib/auth'
import { updatePlayerProfile } from '../../lib/avatars'
import { generateGuestIdentities } from '../../lib/guestIdentity'
import { IDENTITY_GATE_ERROR, IDENTITY_GATE_PROMPT, IDENTITY_GATE_SCOPE_HINT } from '../../lib/playerCopy'

/**
 * The whole identity in one pick, for someone who has neither half. Each row is
 * a name and the face that goes with it, so a visitor is one click from playing.
 */
export default function GuestIdentityPicker({ user, onSaved, onBusyChange }) {
  const [identities] = useState(() => generateGuestIdentities())
  const [pendingName, setPendingName] = useState('')
  const [error, setError] = useState('')

  async function handlePick(identity) {
    if (pendingName) {
      return
    }
    setPendingName(identity.name)
    onBusyChange?.(true)
    setError('')
    try {
      // A visitor with no session at all needs one before the profile can be saved.
      if (!user) {
        await createGuestSession()
      }
      onSaved(await updatePlayerProfile(identity.name, identity.avatarKey))
    } catch (err) {
      setError(err.message || IDENTITY_GATE_ERROR)
    } finally {
      setPendingName('')
      onBusyChange?.(false)
    }
  }

  return (
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
    </>
  )
}
