import { useEffect, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { useActiveIntent } from './useActiveIntent'
import { hasWaitingIntent } from '../../lib/intent'
import { navigateOutOfWaiting } from '../../lib/waiting'
import {
  STOP_LOOKING,
  bannerIntentWaitingHint,
  bannerLiveUpdatesPausedHint,
  formatFormingGapsNeedLine,
  waitingAsRoleLine,
  waitingForGroupLine,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'

/**
 * A refresh on /waiting starts with no intent in hand, so "no queued intent" only
 * becomes a fact once a fetch has actually completed. Redirecting before that
 * would bounce every reload straight back off the page.
 */
function useIntentSettled(loading, authLoading, user) {
  const [settled, setSettled] = useState(false)
  const sawLoading = useRef(false)

  useEffect(() => {
    if (authLoading) {
      return
    }
    // A signed-out visitor never triggers a fetch — there is nothing to wait for.
    if (!user) {
      setSettled(true)
      return
    }
    if (loading) {
      sawLoading.current = true
    } else if (sawLoading.current) {
      setSettled(true)
    }
  }, [loading, authLoading, user])

  return settled
}

/**
 * The queued state as a page of its own (JQ-197), replacing the banner branch that
 * used to sit above the catalog. It shows what the banner showed — no wait-time
 * estimate, that is JQ-58 — with the room to say it properly.
 */
export default function WaitingPage() {
  const { user, loading: authLoading } = useAuth()
  const { activeIntent, loading, busy, queueWsConnected, leaveError, handleLeave } = useActiveIntent()

  const waiting = hasWaitingIntent(activeIntent)
  const settled = useIntentSettled(loading, authLoading, user)
  const liveUpdatesConnected = !activeIntent?.queueId || !waiting || queueWsConnected

  // Once the queued intent is gone the route is a dead end: the player stopped
  // looking, or a match formed. Both hand control back to where they came from.
  useEffect(() => {
    if (waiting || !settled) {
      return
    }
    navigateOutOfWaiting()
  }, [waiting, settled])

  if (!waiting) {
    return (
      <main className="app-shell waiting-page">
        <p className="status-message" role="status">
          {settled ? 'You are not looking for a group.' : 'Checking your queue…'}
        </p>
      </main>
    )
  }

  const role = activeIntent.queuePathDisplayName?.trim()
  const needLine = formatFormingGapsNeedLine(activeIntent.formingGaps)

  return (
    <main className="app-shell waiting-page">
      <section
        className="waiting-page__card"
        role="region"
        aria-live="polite"
        aria-label="Looking for a group"
      >
        <span className="waiting-page__pulse" aria-hidden="true" />

        <h1 className="waiting-page__title">{activeIntent.gameName}</h1>
        {activeIntent.modeName ? (
          <p className="waiting-page__mode">{activeIntent.modeName}</p>
        ) : null}

        <p className="waiting-page__status">{waitingForGroupLine(activeIntent.queuedCount)}</p>
        {role ? <p className="waiting-page__role">{waitingAsRoleLine(role)}</p> : null}
        {needLine ? <p className="waiting-page__need">{needLine}</p> : null}

        <p className="waiting-page__hint">
          {liveUpdatesConnected ? bannerIntentWaitingHint() : bannerLiveUpdatesPausedHint()}
        </p>

        {leaveError ? (
          <p className="waiting-page__error" role="alert">
            {leaveError}
          </p>
        ) : null}

        <Button
          type="button"
          variant="default"
          className="waiting-page__leave"
          onClick={handleLeave}
          disabled={busy}
        >
          {busy ? '…' : STOP_LOOKING}
        </Button>
      </section>
    </main>
  )
}
