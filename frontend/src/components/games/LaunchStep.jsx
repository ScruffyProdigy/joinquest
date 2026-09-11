import { useCallback, useEffect, useRef, useState } from 'react'
import { playingIntentTitle, resolveIntentLaunchUrl } from '../../lib/intent'
import { LAUNCH_COUNTDOWN_SECONDS, MATCH_FOUND_BEAT_MS, navigateToLaunchUrl } from '../../lib/launch'
import { isTabVisible } from '../../lib/tabVisibility'
import {
  LAUNCH_AUTO_HINT,
  LAUNCH_GAME,
  LAUNCH_HELD_HINT,
  LEAVE_GAME,
  MATCH_FOUND,
  MATCH_IN_PROGRESS,
  READY_TO_LAUNCH,
  REJOIN_AUTO_HINT,
  REJOIN_HELD_HINT,
  REJOIN_MATCH,
  bannerIntentLaunchPendingHint,
  launchCountdownLine,
  waitingPageSubline,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'

/**
 * The moment between a match forming and the player entering the game (JQ-136).
 *
 * "You're in!" marks the event, then a countdown carries the player in. Auto-launch
 * is gated on the tab actually being in front: navigating a tab nobody is watching
 * is hostile, and the player who left is JQ-199's seat-hold problem, not a reason to
 * yank the browser. A held countdown always keeps the manual launch action.
 *
 * `immediate` drops the beat and the countdown for a group that started together.
 * The ceremony is worth it when a match forms out of a queue, because that is news
 * to the player; in a friend room the whole group is already watching the Start
 * button, and a countdown does nothing but strand them behind whoever pressed it.
 * The tab gate stays, because it is about the browser rather than about the wait.
 *
 * `rejoin` is the same step turned around, for a player who already fell out of a
 * live match and came back to the lobby (JQ-261). It keeps the countdown but drops
 * the beat -- a game already in progress is not news -- and, crucially, mints its
 * way in through `onRejoin` when the clock fires rather than travelling on the URL
 * it rendered with. A rejoin token is short-lived by design (JQ-86), so the URL this
 * component was handed may have gone stale while the player was away. `launchUrl`
 * is still read, but only as the signal that the match is provisioned at all.
 */
export default function LaunchStep({
  activeIntent,
  activeTableSeat,
  busy,
  leaveError,
  rejoinError = null,
  rejoining = false,
  onLeave,
  onRejoin,
  immediate = false,
  rejoin = false,
}) {
  const launchUrl = resolveIntentLaunchUrl(activeIntent, activeTableSeat)
  // Nothing to announce when the player already knew they were in a game, so the
  // rejoin step opens past the beat and goes straight to the clock.
  const [ready, setReady] = useState(immediate || rejoin)
  const [secondsLeft, setSecondsLeft] = useState(immediate ? 0 : LAUNCH_COUNTDOWN_SECONDS)
  const [held, setHeld] = useState(false)
  const [leaving, setLeaving] = useState(false)
  // The clock gets one shot. Navigating away made that true on its own, but a
  // rejoin that the server refuses leaves this component mounted and armed, and
  // without the latch every re-render would fire another doomed attempt.
  const enteredRef = useRef(false)

  useEffect(() => {
    if (immediate || rejoin) {
      return undefined
    }
    const timer = window.setTimeout(() => setReady(true), MATCH_FOUND_BEAT_MS)
    return () => window.clearTimeout(timer)
  }, [immediate, rejoin])

  const enterGame = useCallback(() => {
    if (rejoin) {
      void onRejoin?.()
      return
    }
    navigateToLaunchUrl(launchUrl)
  }, [rejoin, onRejoin, launchUrl])

  // No launch URL yet means the provision is still in flight: the clock waits for it
  // rather than running down to a dead end.
  const armed = ready && Boolean(launchUrl) && !held && !leaving
  const counting = armed && secondsLeft > 0

  useEffect(() => {
    if (!counting) {
      return undefined
    }
    if (!isTabVisible()) {
      setHeld(true)
      return undefined
    }
    const timer = window.setInterval(() => {
      if (!isTabVisible()) {
        setHeld(true)
        return
      }
      setSecondsLeft((prev) => Math.max(0, prev - 1))
    }, 1000)
    return () => window.clearInterval(timer)
  }, [counting])

  useEffect(() => {
    if (!armed || secondsLeft > 0 || enteredRef.current) {
      return
    }
    // The last word on whether to navigate, so that a launch with no countdown in
    // front of it is gated on the tab too, not only the ticks of one.
    if (!isTabVisible()) {
      setHeld(true)
      return
    }
    enteredRef.current = true
    enterGame()
  }, [armed, secondsLeft, enterGame])

  const subline = waitingPageSubline(
    activeIntent?.gameName ?? activeTableSeat?.gameName,
    activeIntent?.modeName ?? activeTableSeat?.modeName,
    activeIntent?.seatDisplayName ||
      activeIntent?.queuePathDisplayName ||
      activeTableSeat?.seatDisplayName,
    activeIntent?.selectedOptions,
  )

  let status
  if (!launchUrl) {
    status = bannerIntentLaunchPendingHint()
  } else if (held || leaving) {
    status = rejoin ? REJOIN_HELD_HINT : LAUNCH_HELD_HINT
  } else {
    status = rejoin ? REJOIN_AUTO_HINT : LAUNCH_AUTO_HINT
  }

  let title
  if (rejoin) {
    title = MATCH_IN_PROGRESS
  } else {
    title = ready ? READY_TO_LAUNCH : MATCH_FOUND
  }

  return (
    <section
      className="waiting-page__card launch-step"
      role="region"
      aria-live="polite"
      aria-label={rejoin ? MATCH_IN_PROGRESS : 'Match found'}
    >
      <span className="launch-step__mark" aria-hidden="true">
        ✓
      </span>

      <h1 className="waiting-page__title">{title}</h1>
      <p className="waiting-page__subline">{subline || playingIntentTitle(activeIntent, activeTableSeat)}</p>

      {counting ? (
        // The seconds change every tick; announcing each one would talk over the
        // hint that actually tells the player what is about to happen.
        <p className="waiting-page__status launch-step__countdown" aria-live="off">
          {launchCountdownLine(secondsLeft)}
        </p>
      ) : null}

      <p className="waiting-page__hint">{status}</p>

      {rejoinError ? (
        <p className="waiting-page__error" role="alert">
          {rejoinError}
        </p>
      ) : null}

      {leaveError ? (
        <p className="waiting-page__error" role="alert">
          {leaveError}
        </p>
      ) : null}

      <div className="launch-step__actions">
        {launchUrl ? (
          // A button rather than a link, in both directions. The rejoin URL does not
          // exist until this is pressed, and the launch URL carries a token that has
          // no business sitting in the DOM waiting to be copied or to go stale
          // (JQ-261).
          <Button
            type="button"
            variant="default"
            disabled={rejoining}
            onClick={() => {
              enteredRef.current = true
              enterGame()
            }}
          >
            {rejoining ? '…' : rejoin ? REJOIN_MATCH : LAUNCH_GAME}
          </Button>
        ) : null}
        <Button
          type="button"
          variant="secondary"
          disabled={busy}
          onClick={() => {
            // Nothing carries a player who is on their way out.
            setLeaving(true)
            void onLeave?.()
          }}
        >
          {busy ? '…' : LEAVE_GAME}
        </Button>
      </div>
    </section>
  )
}
