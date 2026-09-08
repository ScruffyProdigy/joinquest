import { useEffect, useRef } from 'react'
import { leaveQueue, leaveQueueOnExit } from '../../lib/queue'
import { parseWaitingRoute } from '../../lib/waiting'

/**
 * The waiting page *is* the queue: leaving it gives up the player's place, as the
 * Figma Make demo says in as many words ("You'll lose your spot and have to start
 * over"). Until JQ-198 can notify a player who is not looking at the page, staying
 * queued in the background would mean a match forming for someone who never finds out.
 *
 * Two exits, deliberately disjoint:
 *
 * - Leaving the route (Back, a link, anything through navigateTo, which dispatches
 *   popstate). Keyed off the route rather than unmount on purpose: StrictMode
 *   mounts, unmounts and remounts in development, and an unmount-cleanup leave
 *   would drop every dev player from the queue the moment the page rendered.
 * - The document going away (refresh, tab close), where no cleanup runs at all and
 *   the request has to survive on its own.
 *
 * Both check that the intent is *still* queued, so the ordinary exits — the player
 * confirmed Stop looking, or a match formed — do not leave a second time.
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
      void leaveQueue(queueId).catch(() => {
        // The player has already navigated on; there is no surface left to report to.
      })
    }

    function onPageHide() {
      const { queueId, waiting } = stateRef.current
      if (!waiting) {
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
