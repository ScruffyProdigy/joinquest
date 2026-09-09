import { useEffect, useState } from 'react'
import { playingIntentTitle, resolveIntentLaunchUrl } from '../../lib/intent'
import { LAUNCH_COUNTDOWN_SECONDS, MATCH_FOUND_BEAT_MS, navigateToLaunchUrl } from '../../lib/launch'
import { isTabVisible } from '../../lib/tabVisibility'
import {
  LAUNCH_AUTO_HINT,
  LAUNCH_GAME,
  LAUNCH_HELD_HINT,
  LEAVE_GAME,
  MATCH_FOUND,
  READY_TO_LAUNCH,
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
 * yank the browser. A held countdown always keeps the manual launch link.
 */
export default function LaunchStep({ activeIntent, activeTableSeat, busy, leaveError, onLeave }) {
  const launchUrl = resolveIntentLaunchUrl(activeIntent, activeTableSeat)
  const [ready, setReady] = useState(false)
  const [secondsLeft, setSecondsLeft] = useState(LAUNCH_COUNTDOWN_SECONDS)
  const [held, setHeld] = useState(false)
  const [leaving, setLeaving] = useState(false)

  useEffect(() => {
    const timer = window.setTimeout(() => setReady(true), MATCH_FOUND_BEAT_MS)
    return () => window.clearTimeout(timer)
  }, [])

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
    if (!armed || secondsLeft > 0) {
      return
    }
    navigateToLaunchUrl(launchUrl)
  }, [armed, secondsLeft, launchUrl])

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
    status = LAUNCH_HELD_HINT
  } else {
    status = LAUNCH_AUTO_HINT
  }

  return (
    <section
      className="waiting-page__card launch-step"
      role="region"
      aria-live="polite"
      aria-label="Match found"
    >
      <span className="launch-step__mark" aria-hidden="true">
        ✓
      </span>

      <h1 className="waiting-page__title">{ready ? READY_TO_LAUNCH : MATCH_FOUND}</h1>
      <p className="waiting-page__subline">{subline || playingIntentTitle(activeIntent, activeTableSeat)}</p>

      {counting ? (
        // The seconds change every tick; announcing each one would talk over the
        // hint that actually tells the player what is about to happen.
        <p className="waiting-page__status launch-step__countdown" aria-live="off">
          {launchCountdownLine(secondsLeft)}
        </p>
      ) : null}

      <p className="waiting-page__hint">{status}</p>

      {leaveError ? (
        <p className="waiting-page__error" role="alert">
          {leaveError}
        </p>
      ) : null}

      <div className="launch-step__actions">
        {launchUrl ? (
          <Button asChild variant="default">
            {/* Slot plumbing, not page markup (JQ-72): Button owns the styling and
                the anchor supplies only the href, so this stays a real link. */}
            <a href={launchUrl}>{LAUNCH_GAME}</a>
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
