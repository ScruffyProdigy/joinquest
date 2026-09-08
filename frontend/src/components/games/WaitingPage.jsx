import { useEffect, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { useActiveIntent } from './useActiveIntent'
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

/**
 * A refresh on /waiting starts with no intent in hand, so "no queued intent" only
 * becomes a fact once a fetch has actually completed. Redirecting before that
 * would bounce every reload straight off the page.
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
 * The queued state as a page of its own (JQ-197), following the Figma Make demo's
 * queue screen: heading, one subline naming the game and role, and the way out.
 *
 * The demo's percentage ring and its three threshold-lit steps are driven by a fake
 * timer with no queue behind it, so they do not port. Production has real signals the
 * demo lacks — how many are looking, which roles are still missing — and those take
 * the space the ring occupied. Wait-time estimates stay JQ-58's.
 */
export default function WaitingPage() {
  const { user, loading: authLoading } = useAuth()
  const { activeIntent, loading, busy, queueWsConnected, leaveError, handleLeave } = useActiveIntent()
  const [confirmingLeave, setConfirmingLeave] = useState(false)

  const waiting = hasWaitingIntent(activeIntent)
  const settled = useIntentSettled(loading, authLoading, user)
  const liveUpdatesConnected = !activeIntent?.queueId || !waiting || queueWsConnected

  useLeaveQueueOnExit(activeIntent)

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

  const subline = waitingPageSubline(
    activeIntent.gameName,
    activeIntent.modeName,
    activeIntent.queuePathDisplayName,
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

      {/*
        Leaving costs the player their place, so the button asks first — the demo does
        the same. Browser Back cannot be intercepted this way and leaves without asking;
        useLeaveQueueOnExit still gives up the seat so the queue never holds a ghost.
      */}
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
