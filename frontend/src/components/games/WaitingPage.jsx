import { useEffect, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import LaunchStep from './LaunchStep'
import NotifyMeControl from './NotifyMeControl'
import { useLeaveQueueOnExit } from './useLeaveQueueOnExit'
import { hasGroupWaitingIntent, hasReadyToPlayIntent, hasWaitingIntent } from '../../lib/intent'
import { cancelTableBackfill, discardTable, leaveTable } from '../../lib/tables'
import { navigateOutOfWaiting } from '../../lib/waiting'
import { navigateTo } from '../../lib/usePathname'
import {
  FINDING_PLAYERS,
  LEAVE_QUEUE_BODY,
  LEAVE_QUEUE_CONFIRM,
  LEAVE_QUEUE_TITLE,
  GROUP_WAIT_KEEP_LOOKING,
  GROUP_WAIT_LEAVE,
  GROUP_WAIT_LEAVE_BODY,
  GROUP_WAIT_LEAVE_CONFIRM,
  GROUP_WAIT_LEAVE_TITLE,
  GROUP_WAIT_STAY,
  GROUP_WAIT_STOP_BODY,
  GROUP_WAIT_STOP_CONFIRM,
  GROUP_WAIT_STOP_TITLE,
  STAY_IN_QUEUE,
  STOP_FINDING,
  WAITING_REGION_LABEL,
  bannerIntentWaitingHint,
  bannerLiveUpdatesPausedHint,
  waitingPageNotifiedHint,
  formatFormingGapsNeedLine,
  estimatedWaitLine,
  GROUP_WAIT_OWNER_HINT,
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
  const { activeIntent, activeTableSeat, loading, busy, queueWsConnected, leaveError, handleLeave } =
    intent
  const [confirmingLeave, setConfirmingLeave] = useState(false)
  /*
    A wait that belongs to a table, not to this player (JQ-137). The whole group was
    queued together and sent here together, so the page's ordinary exits are wrong: a
    per-player leaveQueue cancels the party and leaves everyone else in the queue as
    strangers, which is the one outcome the group control exists to prevent.

    So the way out is the table's own cancel, and it belongs to the king — the same
    person who committed the group in the first place. `canCancelBackfill` is the
    server's per-viewer answer, because everyone seated is looking at this screen and
    only one of them may stop it.
  */
  const groupWait = hasGroupWaitingIntent(activeIntent, activeTableSeat)
  const mayStopGroup = groupWait && activeTableSeat?.canCancelBackfill === true
  const [groupStopError, setGroupStopError] = useState('')
  const [stoppingGroup, setStoppingGroup] = useState(false)
  const [confirmingGroupExit, setConfirmingGroupExit] = useState(false)

  async function stopLookingForGroup() {
    setGroupStopError('')
    setStoppingGroup(true)
    try {
      await cancelTableBackfill(activeTableSeat.tableId)
      // Back where the group came from. `navigateOutOfWaiting` already remembers it,
      // and for this page that is always the group screen.
      navigateOutOfWaiting()
    } catch (err) {
      setGroupStopError(err.message || 'Could not stop looking. Try again.')
    } finally {
      setStoppingGroup(false)
    }
  }

  /*
    One player out, everybody else still looking (JQ-137). Deliberately not the queue's
    own leave: that cancels the party and would re-enter the others as strangers. The
    table's leave releases only this player's chair, so the group keeps the seats it
    already holds and a stranger takes the one that opened.
  */
  async function leaveTheGroup() {
    setGroupStopError('')
    setStoppingGroup(true)
    try {
      await leaveTable(activeTableSeat.tableId)
      // Nothing left to leave behind if they were the last one; refused, and ignored,
      // while anybody is still seated.
      await discardTable(activeTableSeat.tableId).catch(() => {})
      // The catalog, not the group screen — they are not in that group any more.
      navigateTo('/', { replace: true })
    } catch (err) {
      setGroupStopError(err.message || 'Could not leave the group. Try again.')
    } finally {
      setStoppingGroup(false)
    }
  }
  // Whether a verified push subscription is in force. Owned here rather than
  // inside the control, because opting in changes the page around it too — a
  // screen that still says "we will notify you here" is not permission to
  // leave it.
  const [notified, setNotified] = useState(false)

  const waiting = hasWaitingIntent(activeIntent)
  // A formed match stays on this page: the launch moment is the point of it (JQ-136).
  const readyToPlay = hasReadyToPlayIntent(activeIntent, activeTableSeat)
  const settled = useIntentSettled(loading, authLoading, user)
  const liveUpdatesConnected = !activeIntent?.queueId || !waiting || queueWsConnected

  // Never for a group: walking back to the group screen is not leaving the queue, and
  // spending one member's place would quietly break the party for the others.
  useLeaveQueueOnExit(groupWait ? null : activeIntent, () => setConfirmingLeave(true))

  // No intent at all means this route is a dead end — hand control back.
  useEffect(() => {
    if (waiting || readyToPlay || !settled) {
      return
    }
    navigateOutOfWaiting()
  }, [waiting, readyToPlay, settled])

  if (readyToPlay) {
    return (
      <main className="app-shell waiting-page">
        <LaunchStep
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          busy={busy}
          leaveError={leaveError}
          onLeave={handleLeave}
        />
      </main>
    )
  }

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
  const waitLine = estimatedWaitLine(activeIntent.estimatedWaitSeconds)

  return (
    <main className="app-shell waiting-page">
      <section
        className="waiting-page__card"
        role="region"
        aria-live="polite"
        aria-label={WAITING_REGION_LABEL}
      >
        <span className="waiting-page__pulse" aria-hidden="true" />

        <h1 className="waiting-page__title">{FINDING_PLAYERS}</h1>
        <p className="waiting-page__subline">{subline}</p>

        <p className="waiting-page__status">{waitingForGroupLine(activeIntent.queuedCount)}</p>
        {needLine ? <p className="waiting-page__need">{needLine}</p> : null}
        {waitLine ? <p className="waiting-page__estimate">{waitLine}</p> : null}

        <p className="waiting-page__hint">
          {notified && liveUpdatesConnected
            ? waitingPageNotifiedHint()
            : liveUpdatesConnected
              ? bannerIntentWaitingHint()
              : bannerLiveUpdatesPausedHint()}
        </p>

        {leaveError ? (
          <p className="waiting-page__error" role="alert">
            {leaveError}
          </p>
        ) : null}

        {/* Offered from the moment the player joins, not after a delay: someone
            who pockets their phone at 8s must already have been given the
            option. The timer inside only promotes it. */}
        <NotifyMeControl
          disabled={busy}
          estimatedWaitSeconds={activeIntent.estimatedWaitSeconds}
          onReachableChange={setNotified}
        />

        {groupWait && !mayStopGroup ? (
          // Absent rather than disabled, for the same reason it is on the group screen:
          // a greyed-out Stop invites a player to wonder what they did wrong.
          <p className="text-xs text-muted-foreground" role="status">
            {GROUP_WAIT_OWNER_HINT}
          </p>
        ) : (
          <Button
            type="button"
            variant="secondary"
            className="waiting-page__leave"
            onClick={() => setConfirmingLeave(true)}
            disabled={busy || stoppingGroup}
          >
            {busy || stoppingGroup ? '…' : STOP_FINDING}
          </Button>
        )}
        {/* Anyone may go it alone, the king included — they are leaving, not deciding
            for everybody, and the next-earliest seated player inherits the role. */}
        {groupWait ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={busy || stoppingGroup}
            onClick={() => setConfirmingGroupExit(true)}
          >
            {GROUP_WAIT_LEAVE}
          </Button>
        ) : null}
        {groupStopError ? (
          <p className="status-message status-message-error" role="status">
            {groupStopError}
          </p>
        ) : null}
      </section>

      <Sheet open={confirmingGroupExit} onOpenChange={setConfirmingGroupExit}>
        <SheetContent side="bottom" aria-label={GROUP_WAIT_LEAVE_TITLE}>
          <SheetTitle>{GROUP_WAIT_LEAVE_TITLE}</SheetTitle>
          <p className="waiting-page__confirm-body">{GROUP_WAIT_LEAVE_BODY}</p>
          <div className="waiting-page__confirm-actions">
            <Button
              type="button"
              variant="default"
              disabled={busy || stoppingGroup}
              onClick={() => {
                setConfirmingGroupExit(false)
                void leaveTheGroup()
              }}
            >
              {GROUP_WAIT_LEAVE_CONFIRM}
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setConfirmingGroupExit(false)}
            >
              {GROUP_WAIT_STAY}
            </Button>
          </div>
        </SheetContent>
      </Sheet>

      {/* Leaving costs the player their place, so every way out asks first — the
          button here, and a back gesture, which `useLeaveQueueOnExit` undoes so it
          arrives at this same sheet rather than leaving on its own (JQ-218). */}
      <Sheet open={confirmingLeave} onOpenChange={setConfirmingLeave}>
        <SheetContent side="bottom" aria-label={groupWait ? GROUP_WAIT_STOP_TITLE : LEAVE_QUEUE_TITLE}>
          <SheetTitle>{groupWait ? GROUP_WAIT_STOP_TITLE : LEAVE_QUEUE_TITLE}</SheetTitle>
          <p className="waiting-page__confirm-body">
            {groupWait ? GROUP_WAIT_STOP_BODY : LEAVE_QUEUE_BODY}
          </p>
          <div className="waiting-page__confirm-actions">
            <Button
              type="button"
              variant="default"
              disabled={busy || stoppingGroup}
              onClick={() => {
                setConfirmingLeave(false)
                if (groupWait) {
                  void stopLookingForGroup()
                  return
                }
                void handleLeave()
              }}
            >
              {groupWait ? GROUP_WAIT_STOP_CONFIRM : LEAVE_QUEUE_CONFIRM}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setConfirmingLeave(false)}>
              {groupWait ? GROUP_WAIT_KEEP_LOOKING : STAY_IN_QUEUE}
            </Button>
          </div>
        </SheetContent>
      </Sheet>
    </main>
  )
}
