import { useState } from 'react'
import { updatePlayerProfile } from '../../lib/avatars'
import { generateGuestIdentities } from '../../lib/guestIdentity'
import { AVATAR_PROMPT_ERROR, AVATAR_PROMPT_SCOPE_HINT, playingAsLine } from '../../lib/playerCopy'
import { chosenDisplayName } from '../../lib/viewer'

/**
 * The mirror of DisplayNamePrompt: a name on file and no face to go with it.
 * The sigils are drawn the same way the guest picker draws them, minus their
 * generated names — the player keeps the one they already chose.
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

      <ul className="grid grid-cols-3 gap-3 sm:grid-cols-6" role="list">
        {identities.map((identity) => (
          <li key={identity.avatarKey}>
            <button
              type="button"
              className="flex w-full flex-col items-center gap-1 rounded-2xl border border-border bg-muted/40 px-2 py-3 transition-colors hover:bg-muted disabled:opacity-60"
              disabled={Boolean(pendingKey)}
              aria-label={identity.label}
              onClick={() => void handlePick(identity)}
            >
              <img src={identity.imageUrl} alt="" className="size-10 shrink-0 rounded-full" />
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
