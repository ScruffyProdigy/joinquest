import { useEffect, useRef } from 'react'
import { lobbyDebug } from '../../lib/lobbyDebug'
import { leaveQueue, leaveQueueOnExit } from '../../lib/queue'
import { navigateToWaiting, parseWaitingRoute } from '../../lib/waiting'

/**
 * Leaving the waiting page gives up the player's place in the queue.
 *
 * Two exits: leaving the route (popstate), and the document being torn down
 * (pagehide), where no cleanup runs and the request has to survive on its own.
 * Both check the intent is still WAITING so the ordinary exits — Stop looking, a
 * match forming — do not leave twice.
 */
export function useLeaveQueueOnExit(activeIntent) {
  const stateRef = useRef({ queueId: null, waiting: false })
  stateRef.current = {
    queueId: activeIntent?.queueId ?? null,
    waiting: activeIntent?.status === 'WAITING',
  }

  useEffect(() => {
    function onRouteChange() {
      const { queueId, waiting } = stateRef.current
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
