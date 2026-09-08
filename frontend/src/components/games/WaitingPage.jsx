import { useEffect, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { useLeaveQueueOnExit } from './useLeaveQueueOnExit'
import { hasWaitingIntent } from '../../lib/intent'
import { navigateOutOfWaiting } from '../../lib/waiting'
import {
  FINDING_PLAYERS,
  LEAVE_QUEUE_BODY,
  LEAVE_QUEUE_CONFIRM,
  LEAVE_QUEUE_TITLE,
  STAY_IN_QUEUE,
  STOP_LOOKING,
  bannerIntentWaitingHint,
  bannerLiveUpdatesPausedHint,
  formatFormingGapsNeedLine,
  waitingForGroupLine,
  waitingPageSubline,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'
import { Sheet, SheetContent, SheetTitle } from '../ui/sheet'

/** True once an intent fetch has actually completed, so a reload is not bounced off
 * the page before the first result lands. */
function useIntentSettled(loading, authLoading, user) {
  const [settled, setSettled] = useState(false)
  const sawLoading = useRef(false)

  useEffect(() => {
    if (authLoading) {
      return
    }
    // A signed-out visitor never triggers a fetch.
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

/** The queued state: what the player is waiting for, how it is going, and the way out. */
export default function WaitingPage({ intent }) {
  const { user, loading: authLoading } = useAuth()
  const { activeIntent, loading, busy, queueWsConnected, leaveError, handleLeave } = intent
  const [confirmingLeave, setConfirmingLeave] = useState(false)

  const waiting = hasWaitingIntent(activeIntent)
  const settled = useIntentSettled(loading, authLoading, user)
  const liveUpdatesConnected = !activeIntent?.queueId || !waiting || queueWsConnected

  useLeaveQueueOnExit(activeIntent)

  // No queued intent means this route is a dead end — hand control back.
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

  const subline = waitingPageSubline(
    activeIntent.gameName,
    activeIntent.modeName,
    activeIntent.queuePathDisplayName,
    activeIntent.selectedOptions,
  )
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

        <h1 className="waiting-page__title">{FINDING_PLAYERS}</h1>
        <p className="waiting-page__subline">{subline}</p>

        <p className="waiting-page__status">{waitingForGroupLine(activeIntent.queuedCount)}</p>
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
          variant="secondary"
          className="waiting-page__leave"
          onClick={() => setConfirmingLeave(true)}
          disabled={busy}
        >
          {busy ? '…' : STOP_LOOKING}
        </Button>
      </section>

      {/* Leaving costs the player their place, so the button asks first. Browser Back
          cannot be intercepted this way and leaves without asking. */}
      <Sheet open={confirmingLeave} onOpenChange={setConfirmingLeave}>
        <SheetContent side="bottom" aria-label={LEAVE_QUEUE_TITLE}>
          <SheetTitle>{LEAVE_QUEUE_TITLE}</SheetTitle>
          <p className="waiting-page__confirm-body">{LEAVE_QUEUE_BODY}</p>
          <div className="waiting-page__confirm-actions">
            <Button
              type="button"
              variant="default"
              disabled={busy}
              onClick={() => {
                setConfirmingLeave(false)
                void handleLeave()
              }}
            >
              {LEAVE_QUEUE_CONFIRM}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setConfirmingLeave(false)}>
              {STAY_IN_QUEUE}
            </Button>
          </div>
        </SheetContent>
      </Sheet>
    </main>
  )
}
