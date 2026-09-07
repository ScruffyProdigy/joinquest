import { useState } from 'react'
import { updatePlayerProfile } from '../../lib/avatars'
import {
  NAME_PROMPT_ERROR,
  NAME_PROMPT_KEEPING_AVATAR,
  NAME_PROMPT_LABEL,
  NAME_PROMPT_PLACEHOLDER,
  NAME_PROMPT_SAVING,
  NAME_PROMPT_SCOPE_HINT,
  NAME_PROMPT_SUBMIT,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import PlayerAvatar from './PlayerAvatar'

/**
 * For an account whose name was never chosen — every account migrated by
 * 000045, and any signup that derives a name instead of asking for one. The
 * avatar they already have is shown rather than re-collected: saving with no
 * avatar key leaves it exactly as it is, spirit animal included.
 */
export default function DisplayNamePrompt({ user, onSaved, onBusyChange }) {
  const [displayName, setDisplayName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const trimmed = displayName.trim()

  async function handleSubmit(event) {
    event.preventDefault()
    if (!trimmed || busy) {
      return
    }
    setBusy(true)
    onBusyChange?.(true)
    setError('')
    try {
      onSaved(await updatePlayerProfile(trimmed, null))
    } catch (err) {
      setError(err.message || NAME_PROMPT_ERROR)
    } finally {
      setBusy(false)
      onBusyChange?.(false)
    }
  }

  return (
    <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
      <div className="flex items-center justify-center gap-3">
        <PlayerAvatar user={user} size="md" />
        <p className="text-sm text-muted-foreground">{NAME_PROMPT_KEEPING_AVATAR}</p>
      </div>

      <div className="flex flex-col gap-2">
        <label htmlFor="identity-display-name" className="text-sm font-medium text-foreground">
          {NAME_PROMPT_LABEL}
        </label>
        <Input
          id="identity-display-name"
          type="text"
          maxLength={100}
          autoComplete="nickname"
          placeholder={NAME_PROMPT_PLACEHOLDER}
          value={displayName}
          disabled={busy}
          onChange={(event) => setDisplayName(event.target.value)}
        />
        <p className="text-2xs text-muted-foreground">{NAME_PROMPT_SCOPE_HINT}</p>
      </div>

      {error ? (
        <p className="status-message status-message-error" role="status">
          {error}
        </p>
      ) : null}

      <Button type="submit" disabled={!trimmed || busy}>
        {busy ? NAME_PROMPT_SAVING : NAME_PROMPT_SUBMIT}
      </Button>
    </form>
  )
}
