import { useState } from 'react'
import { createGuestSession } from '../../lib/auth'
import { updatePlayerProfile } from '../../lib/avatars'
import { generateGuestIdentities } from '../../lib/guestIdentity'
import { cn } from '../../lib/utils'
import {
  IDENTITY_GATE_ERROR,
  IDENTITY_GATE_PROMPT,
  IDENTITY_GATE_SCOPE_HINT,
  IDENTITY_GATE_STARTING,
  jumpInAsLine,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'
import AvatarChoiceGrid from './AvatarChoiceGrid'

/**
 * The whole identity in one pick, for someone who has neither half. Each row is
 * a name and the face that goes with it, so a visitor is two taps from playing.
 *
 * Two, not one, because that is the prototype: a tap selects, and the identity
 * is only taken when the named button at the bottom of the screen is pressed —
 * or when the row already selected is tapped again. A player who mis-taps on a
 * phone gets to look at the name before it becomes theirs, and the screen gets
 * to say which name that is. The profile editor has always worked this way; the
 * gate was the odd one out, committing on first touch.
 *
 * It renders two of the takeover's three blocks — the picking, and the footer
 * the confirm button rises into — because the footer holds the selection and
 * the prototype keeps it pinned to the bottom edge rather than under the grid.
 * The gate owns the third (the heading) and passes the sign-in offer down, so
 * that the blocks arrive in the order the takeover lays out.
 */
export default function GuestIdentityPicker({ user, onSaved, onBusyChange, signIn }) {
  const [identities] = useState(() => generateGuestIdentities())
  const [selectedKey, setSelectedKey] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const selected = identities.find((identity) => identity.avatarKey === selectedKey)

  /** A tap on the selected row is the confirm gesture; any other row selects. */
  function handlePick(choice) {
    if (busy) {
      return
    }
    if (choice.key === selectedKey) {
      commit(identities.find((identity) => identity.avatarKey === choice.key))
      return
    }
    setError('')
    setSelectedKey(choice.key)
  }

  async function commit(identity) {
    if (!identity || busy) {
      return
    }
    setBusy(true)
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
      setBusy(false)
      onBusyChange?.(false)
    }
  }

  return (
    <>
      <div className="flex w-full flex-1 flex-col justify-center pb-8">
        <div className="flex flex-col items-center gap-1 text-center">
          <p className="font-heading text-base font-bold text-muted-foreground">{IDENTITY_GATE_PROMPT}</p>
          <p className="text-2xs text-muted-foreground">{IDENTITY_GATE_SCOPE_HINT}</p>
        </div>

        <div className="pt-5">
          <AvatarChoiceGrid
            choices={identities.map((identity) => ({
              key: identity.avatarKey,
              avatarUrl: identity.imageUrl,
              displayName: identity.name,
            }))}
            selectedKey={selectedKey}
            disabled={busy}
            onPick={handlePick}
          />
        </div>

        {error ? (
          <p className="status-message status-message-error mt-4" role="status">
            {error}
          </p>
        ) : null}

        {signIn}
      </div>

      {/* The footer keeps its height whether or not anything is in it, so that
          selecting a row moves nothing above it.

          It sticks to the bottom edge once it has something to hold: the whole
          screen fits a tall handset, but on a short one the takeover scrolls, and
          the button that starts the game is the last thing that should be the
          part below the fold. Empty, it stays in the flow — a transparent 56px
          band pinned over the sign-in pill would swallow taps meant for it. */}
      <div
        className={cn(
          'relative h-14 w-full shrink-0 overflow-hidden',
          selected && 'sticky bottom-0 bg-background',
        )}
      >
        {selected ? (
          <Button
            className="absolute inset-0 animate-in slide-in-from-bottom duration-300"
            disabled={busy}
            onClick={() => commit(selected)}
          >
            {busy ? IDENTITY_GATE_STARTING : jumpInAsLine(selected.name)}
          </Button>
        ) : null}
      </div>
    </>
  )
}
