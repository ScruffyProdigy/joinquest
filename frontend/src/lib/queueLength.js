/*
 * Long queue or short queue — the question that decides whether the notify
 * control appears at all.
 *
 * It exists because a notification permission prompt is spent once. A denial is
 * effectively permanent per origin: there is no second ask, no in-product way
 * back, and the player who declined on a fifteen-second wait is the same player
 * we most need to reach on a five-minute one. So the prompt is not offered on a
 * queue short enough that nobody would leave the page anyway.
 */

/**
 * Below this, the wait is over before leaving the page is worth doing.
 *
 * A minute, matching where JQ-198 put the line: "on any wait longer than about
 * a minute, players will switch tabs or leave the site". Under it the control
 * would be offering to solve a problem the player does not have yet, and
 * spending the one prompt we get to do it.
 */
export const LONG_QUEUE_SECONDS = 60

/**
 * True when this wait is long enough to offer the notify control.
 *
 * An unknown estimate counts as long. JQ-58 returns null when throughput is not
 * measurable and there is no usable history — which describes a cold or
 * thinly-populated queue, the case where waits are longest and a player left
 * watching an idle screen is most likely to walk away. Guessing "short" there
 * would withhold the control from exactly the people it is for, and the cost of
 * guessing wrong the other way is one prompt on a queue that happened to pop
 * early.
 */
export function isLongQueue(estimatedWaitSeconds) {
  if (estimatedWaitSeconds === null || estimatedWaitSeconds === undefined) {
    return true
  }
  return estimatedWaitSeconds >= LONG_QUEUE_SECONDS
}
