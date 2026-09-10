import { useState } from 'react'
import { createGuestSession } from '../../lib/auth'
import { updatePlayerProfile } from '../../lib/avatars'
import { generateGuestIdentities } from '../../lib/guestIdentity'
import { IDENTITY_GATE_ERROR, IDENTITY_GATE_PROMPT, IDENTITY_GATE_SCOPE_HINT } from '../../lib/playerCopy'
import AvatarChoiceGrid from './AvatarChoiceGrid'

/**
 * The whole identity in one pick, for someone who has neither half. Each row is
 * a name and the face that goes with it, so a visitor is one click from playing.
 */
export default function GuestIdentityPicker({ user, onSaved, onBusyChange }) {
  const [identities] = useState(() => generateGuestIdentities())
  const [pendingKey, setPendingKey] = useState('')
  const [error, setError] = useState('')

  /** Back from a picked row to the identity it was drawn from. */
  function identityFor(choice) {
    return identities.find((identity) => identity.avatarKey === choice.key)
  }

  async function handlePick(identity) {
    if (pendingKey) {
      return
    }
    setPendingKey(identity.avatarKey)
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
      setPendingKey('')
      onBusyChange?.(false)
    }
  }

  return (
    <>
      <div className="flex flex-col items-center gap-1 text-center">
        <p className="text-sm font-medium text-foreground">{IDENTITY_GATE_PROMPT}</p>
        <p className="text-2xs text-muted-foreground">{IDENTITY_GATE_SCOPE_HINT}</p>
      </div>

      <AvatarChoiceGrid
        choices={identities.map((identity) => ({
          key: identity.avatarKey,
          avatarUrl: identity.imageUrl,
          displayName: identity.name,
        }))}
        busyKey={pendingKey}
        onPick={(choice) => handlePick(identityFor(choice))}
      />

      {error ? (
        <p className="status-message status-message-error" role="status">
          {error}
        </p>
      ) : null}
    </>
  )
}
