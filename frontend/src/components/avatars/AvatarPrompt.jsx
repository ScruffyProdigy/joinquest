import { useState } from 'react'
import { updatePlayerProfile } from '../../lib/avatars'
import { generateGuestIdentities } from '../../lib/guestIdentity'
import { AVATAR_PROMPT_ERROR, AVATAR_PROMPT_SCOPE_HINT, playingAsLine } from '../../lib/playerCopy'
import { chosenDisplayName } from '../../lib/viewer'
import SigilChoiceGrid from './SigilChoiceGrid'

/**
 * The mirror of DisplayNamePrompt: a name on file and no face to go with it.
 * The sigils come from the same SigilChoiceGrid the guest picker uses, so the
 * two prompts are one screen with one row swapped: each face is offered under
 * its colour and animal rather than a generated handle, because the player
 * keeps the name they already chose.
 */
export default function AvatarPrompt({ user, onSaved, onBusyChange }) {
  const [identities] = useState(() => generateGuestIdentities())
  const [pendingKey, setPendingKey] = useState('')
  const [error, setError] = useState('')

  const displayName = chosenDisplayName(user)

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

      <SigilChoiceGrid
        identities={identities}
        pendingKey={pendingKey}
        textOf={(identity) => identity.label}
        labelOf={(identity) => identity.label}
        onPick={handlePick}
      />

      {error ? (
        <p className="status-message status-message-error" role="status">
          {error}
        </p>
      ) : null}
    </>
  )
}
