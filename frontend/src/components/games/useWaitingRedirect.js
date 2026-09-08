import { useEffect } from 'react'
import { hasWaitingIntent } from '../../lib/intent'
import { navigateToWaiting, parseWaitingRoute } from '../../lib/waiting'

/**
 * The queued state has its own page now (JQ-197), so the surfaces that used to
 * carry the queued banner send a queued player to it instead. Without this a
 * player who navigated away while queued would see no sign of the queue at all.
 *
 * Replace rather than push: Back off the waiting page lands here, and a push
 * would bounce straight to /waiting while growing history on every press.
 */
export function useWaitingRedirect(activeIntent) {
  const waiting = hasWaitingIntent(activeIntent)

  useEffect(() => {
    if (!waiting || parseWaitingRoute()) {
      return
    }
    navigateToWaiting({ replace: true })
  }, [waiting])
}
