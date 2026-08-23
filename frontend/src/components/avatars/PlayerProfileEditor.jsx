import { useEffect, useState } from 'react'
import {
  defaultDisplayNameInput,
  fetchStarterAvatars,
  hasExistingAvatar,
  needsProfileSetup,
  updatePlayerProfile,
} from '../../lib/avatars'
import { useAuth } from '../auth/AuthProvider'
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
  const keepingCurrentAvatar = hasExistingAvatar(user) && !selectedKey
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
    <form className="profile-editor" onSubmit={handleSave}>
      <p className="profile-editor__lead">
        {required || needsProfileSetup(user)
          ? 'Choose how others will see you in rooms and at tables.'
          : 'Update your display name and journey icon.'}
      </p>

      <label className="profile-editor__label" htmlFor="profile-display-name">
        Display name
      </label>
      <input
        id="profile-display-name"
        className="profile-editor__input"
        type="text"
        maxLength={100}
        autoComplete="nickname"
        placeholder="Your name"
        value={displayName}
        disabled={busy}
        onChange={(event) => setDisplayName(event.target.value)}
      />

      {keepingCurrentAvatar ? (
        <div className="profile-editor__current-avatar">
          <p className="profile-editor__label">Current icon</p>
          <div className="profile-editor__current-avatar-row">
            <PlayerAvatar user={user} size="md" />
            <p className="profile-editor__hint">
              {user?.avatarSource === 'SPIRIT_ANIMAL'
                ? 'Your spirit animal stays unless you pick a journey icon below.'
                : 'Your current icon stays unless you pick a different one below.'}
            </p>
          </div>
        </div>
      ) : null}

      <p className="profile-editor__label">{keepingCurrentAvatar ? 'Or choose a journey icon' : 'Journey icon'}</p>
      <ul className="avatar-picker__grid" role="list">
        {options.map((option) => {
          const selected = option.key === selectedKey
          return (
            <li key={option.key}>
              <button
                type="button"
                className={`avatar-picker__option${selected ? ' avatar-picker__option--selected' : ''}`}
                disabled={busy}
                aria-pressed={selected}
                aria-label={`${option.name}${selected ? ' (selected)' : ''}`}
                onClick={() => setSelectedKey(option.key)}
              >
                <img src={option.imageUrl} alt="" className="avatar-picker__image" />
                <span className="avatar-picker__name">{option.name}</span>
              </button>
            </li>
          )
        })}
      </ul>

      <div className="profile-editor__actions">
        <button type="submit" className="game-list-button" disabled={!canSave}>
          {busy ? 'Saving…' : 'Save display'}
        </button>
        {onBeginSpiritAnimal ? (
          <button
            type="button"
            className="game-list-button game-list-button-secondary"
            disabled={busy}
            onClick={onBeginSpiritAnimal}
          >
            Find my spirit animal
          </button>
        ) : null}
        {!required && onCancel ? (
          <button type="button" className="game-list-button game-list-button-secondary" disabled={busy} onClick={onCancel}>
            Cancel
          </button>
        ) : null}
      </div>

      {error ? <p className="status-message status-message-error">{error}</p> : null}
    </form>
  )
}
