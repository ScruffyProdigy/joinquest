import { useState } from 'react'
import { updatePlayerProfile } from '../../lib/avatars'
import { generateGuestIdentities } from '../../lib/guestIdentity'
import { AVATAR_PROMPT_ERROR, AVATAR_PROMPT_SCOPE_HINT, playingAsLine } from '../../lib/playerCopy'
import { chosenDisplayName } from '../../lib/viewer'
import AvatarChoiceGrid from './AvatarChoiceGrid'

/**
 * The mirror of DisplayNamePrompt: a name on file and no face to go with it.
 * The sigils come from the same AvatarChoiceGrid the guest picker uses, so the
 * two prompts are one screen with one row swapped: each face is offered under
 * its colour and animal rather than a generated handle, because the player
 * keeps the name they already chose.
 */
export default function AvatarPrompt({ user, onSaved, onBusyChange }) {
  const [identities] = useState(() => generateGuestIdentities())
  const [pendingKey, setPendingKey] = useState('')
  const [error, setError] = useState('')

  const displayName = chosenDisplayName(user)

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
      onSaved(await updatePlayerProfile(displayName, identity.avatarKey))
    } catch (err) {
      setError(err.message || AVATAR_PROMPT_ERROR)
    } finally {
      setPendingKey('')
      onBusyChange?.(false)
    }
  }

  return (
    <>
      <div className="flex flex-col items-center gap-1 text-center">
        <p className="text-sm font-medium text-foreground">{playingAsLine(displayName)}</p>
        <p className="text-2xs text-muted-foreground">{AVATAR_PROMPT_SCOPE_HINT}</p>
      </div>

      <AvatarChoiceGrid
        choices={identities.map((identity) => ({
          key: identity.avatarKey,
          avatarUrl: identity.imageUrl,
          displayName: identity.label,
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
