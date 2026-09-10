import { useEffect, useRef } from 'react'
import { lobbyDebug } from '../../lib/lobbyDebug'
import { leaveQueue, leaveQueueOnExit } from '../../lib/queue'
import { navigateToWaiting, parseWaitingRoute, restoreWaitingRoute } from '../../lib/waiting'
import { isAppNavigation } from '../../lib/usePathname'

/**
 * Leaving the waiting page gives up the player's place in the queue.
 *
 * Two exits: leaving the route (popstate), and the document being torn down
 * (pagehide), where no cleanup runs and the request has to survive on its own.
 * Both check the intent is still WAITING so the ordinary exits — Stop looking, a
 * match forming — do not leave twice.
 *
 * A back gesture is the exception: it does not get to spend the place silently.
 * `onBackGesture` raises the same confirmation the Stop looking button does, and the
 * queue is left only once the player says so (JQ-218).
 */
export function useLeaveQueueOnExit(activeIntent, onBackGesture) {
  const stateRef = useRef({ queueId: null, waiting: false })
  stateRef.current = {
    queueId: activeIntent?.queueId ?? null,
    waiting: activeIntent?.status === 'WAITING',
  }
  // The listeners are bound once, so the current callback has to be reachable
  // through a ref rather than closed over.
  const backGestureRef = useRef(onBackGesture)
  backGestureRef.current = onBackGesture

  useEffect(() => {
    function onRouteChange() {
      const { queueId, waiting } = stateRef.current

      // A pop the browser raised — a Back press, or the iOS edge swipe that is far
      // too easy to catch by accident — is one gesture away from spending a queue
      // place that cannot be got back. Undo it and ask, the same way the button
      // asks. Only the app's own navigations are taken at face value.
      if (waiting && queueId && !isAppNavigation()) {
        restoreWaitingRoute()
        backGestureRef.current?.()
        return
      }

      if (!waiting || !queueId || parseWaitingRoute()) {
        return
      }
      void leaveQueue(queueId).catch((err) => {
        // The leave failed, so the player is still queued. Nothing else surfaces
        // the queued state any more, so put them back on the page that does.
        lobbyDebug('queue:leave-on-exit:failed', { queueId, error: err?.message || String(err) })
        navigateToWaiting({ replace: true })
      })
    }

    function onPageHide(event) {
      const { queueId, waiting } = stateRef.current
      // A persisted page is frozen into the back/forward cache, not destroyed, and
      // comes back with its state intact — only a real teardown gives up the queue.
      if (!waiting || event.persisted) {
        return
      }
      leaveQueueOnExit(queueId)
    }

    window.addEventListener('popstate', onRouteChange)
    window.addEventListener('pagehide', onPageHide)
    return () => {
      window.removeEventListener('popstate', onRouteChange)
      window.removeEventListener('pagehide', onPageHide)
    }
  }, [])
}
