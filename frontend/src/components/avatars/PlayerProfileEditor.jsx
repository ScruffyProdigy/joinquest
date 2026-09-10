import { useEffect, useState } from 'react'
import { defaultDisplayNameInput, fetchStarterAvatars, updatePlayerProfile } from '../../lib/avatars'
import { hasChosenAvatar, needsIdentity } from '../../lib/viewer'
import { useAuth } from '../auth/AuthProvider'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import AvatarChoiceGrid from './AvatarChoiceGrid'
import PlayerAvatar from './PlayerAvatar'

export default function PlayerProfileEditor({ user, required = false, onSaved, onCancel, onBeginSpiritAnimal }) {
  const { acceptSessionUser } = useAuth()
  const [options, setOptions] = useState([])
  const [displayName, setDisplayName] = useState(() => defaultDisplayNameInput(user))
  const [selectedKey, setSelectedKey] = useState(user?.avatarKey || '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setDisplayName(defaultDisplayNameInput(user))
    setSelectedKey(user?.avatarKey || '')
  }, [user])

  useEffect(() => {
    let cancelled = false
    void fetchStarterAvatars().then((items) => {
      if (!cancelled) {
        setOptions(items)
      }
    })
    return () => {
      cancelled = true
    }
  }, [])

  const trimmedName = displayName.trim()
  const keepingCurrentAvatar = hasChosenAvatar(user) && !selectedKey
  const canSave = Boolean(trimmedName && !busy && (selectedKey || keepingCurrentAvatar))

  async function handleSave(event) {
    event.preventDefault()
    if (!canSave) {
      return
    }
    setBusy(true)
    setError('')
    try {
      const updated = await updatePlayerProfile(trimmedName, selectedKey || null)
      acceptSessionUser(updated)
      onSaved?.(updated)
    } catch (err) {
      setError(err.message || 'Could not save your display.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="flex flex-col gap-4" onSubmit={handleSave}>
      <p className="text-sm text-muted-foreground">
        {required || needsIdentity(user)
          ? 'Choose how others will see you in rooms and at tables.'
          : 'Update your display name and journey icon.'}
      </p>

      <div className="flex flex-col gap-2">
        <label htmlFor="profile-display-name" className="text-sm font-medium text-muted-foreground">
          Display name
        </label>
        <Input
          id="profile-display-name"
          type="text"
          maxLength={100}
          autoComplete="nickname"
          placeholder="Your name"
          value={displayName}
          disabled={busy}
          onChange={(event) => setDisplayName(event.target.value)}
        />
      </div>

      {keepingCurrentAvatar ? (
        <div className="flex flex-col gap-2">
          <p className="text-sm font-medium text-muted-foreground">Current icon</p>
          <div className="flex items-center gap-3">
            <PlayerAvatar user={user} size="md" />
            <p className="text-sm text-muted-foreground">
              {user?.avatarSource === 'SPIRIT_ANIMAL'
                ? 'Your spirit animal stays unless you pick a journey icon below.'
                : 'Your current icon stays unless you pick a different one below.'}
            </p>
          </div>
        </div>
      ) : null}

      <div className="flex flex-col gap-2">
        <p className="text-sm font-medium text-muted-foreground">
          {keepingCurrentAvatar ? 'Or choose a journey icon' : 'Journey icon'}
        </p>
        <AvatarChoiceGrid
          choices={options.map((option) => ({
            key: option.key,
            avatarUrl: option.imageUrl,
            displayName: option.name,
          }))}
          selectedKey={selectedKey}
          disabled={busy}
          onPick={(choice) => setSelectedKey(choice.key)}
        />
      </div>

      <div className="flex flex-wrap gap-3">
        <Button type="submit" disabled={!canSave}>
          {busy ? 'Saving…' : 'Save display'}
        </Button>
        {onBeginSpiritAnimal ? (
          <Button type="button" variant="secondary" disabled={busy} onClick={onBeginSpiritAnimal}>
            Find my spirit animal
          </Button>
        ) : null}
        {!required && onCancel ? (
          <Button type="button" variant="secondary" disabled={busy} onClick={onCancel}>
            Cancel
          </Button>
        ) : null}
      </div>

      {error ? <p className="status-message status-message-error">{error}</p> : null}
    </form>
  )
}
