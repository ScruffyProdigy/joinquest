import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { isIdentityRequiredError } from '../../lib/graphql'
import { onIdentityRequired } from '../../lib/identityPrompt'
import { needsIdentity } from '../../lib/viewer'
import { useAuth } from '../auth/AuthProvider'
import IdentityGate from './IdentityGate'

/**
 * Owns the identity prompt for the whole app.
 *
 * Browsing is open to anyone; the name and avatar are only asked for at the
 * moment someone acts on an intent to play — joining a queue, creating a
 * private game, sitting at a seat, opening a room invite. Wrap that action in
 * `requireIdentity` and it survives the prompt: the picker goes up, and once
 * the profile saves the original action runs on its own, so one click is still
 * one click.
 *
 * Without a provider the action simply runs. That keeps isolated component
 * tests and any surface mounted outside the shell working — the backend still
 * rejects a play-entry mutation from a nameless caller (ErrIdentityRequired),
 * which is what put the prompt here in the first place.
 */
const IdentityPromptContext = createContext({
  requireIdentity: (action) => Promise.resolve().then(action),
  promptIdentity: () => {},
})

export function useIdentityPrompt() {
  return useContext(IdentityPromptContext)
}

/**
 * Raise the prompt as soon as a surface is reached that is itself an intent to
 * play. A room invite is the one of these: the link was an acceptance, so the
 * visitor is asked before they land in the room.
 */
export function useIdentityPromptOnMount(active) {
  const { promptIdentity } = useIdentityPrompt()
  useEffect(() => {
    if (active) {
      promptIdentity()
    }
  }, [active, promptIdentity])
}

export function IdentityPromptProvider({ children }) {
  const { user, loading, refreshSession } = useAuth()
  const [open, setOpen] = useState(false)
  // The action waiting on the prompt, with the promise handed back to whoever
  // asked for it. At most one — the picker blocks the page, so nothing else
  // can start while it is up.
  const pendingRef = useRef(null)

  const promptIdentity = useCallback(() => {
    setOpen(true)
  }, [])

  const waitForIdentity = useCallback(
    (action) =>
      new Promise((resolve, reject) => {
        pendingRef.current = { action, resolve, reject }
        setOpen(true)
      }),
    [],
  )

  const requireIdentity = useCallback(
    async (action) => {
      if (needsIdentity(user)) {
        return waitForIdentity(action)
      }
      try {
        return await action()
      } catch (err) {
        if (!isIdentityRequiredError(err?.message)) {
          throw err
        }
        // The session looked complete to us and the backend disagreed, so
        // re-read it before answering — a name can be dropped server-side
        // (JQ-182), and then the picker is exactly what is needed. If the
        // session really is complete the action is simply retried once; the
        // replay runs directly, so a second rejection reaches the caller rather
        // than reopening the picker.
        await refreshSession({ silent: true })
        return waitForIdentity(action)
      }
    },
    [user, refreshSession, waitForIdentity],
  )

  // A rejection from a call that was not wrapped still opens the picker; there
  // is just nothing to replay afterwards.
  useEffect(() => onIdentityRequired(() => setOpen(true)), [])

  useEffect(() => {
    if (!open || loading || needsIdentity(user)) {
      return
    }
    setOpen(false)
    const pending = pendingRef.current
    pendingRef.current = null
    if (!pending) {
      return
    }
    Promise.resolve().then(pending.action).then(pending.resolve, pending.reject)
  }, [open, loading, user])

  const value = useMemo(
    () => ({ requireIdentity, promptIdentity }),
    [requireIdentity, promptIdentity],
  )

  return (
    <IdentityPromptContext.Provider value={value}>
      {children}
      {open ? <IdentityGate /> : null}
    </IdentityPromptContext.Provider>
  )
}

export default IdentityPromptProvider
