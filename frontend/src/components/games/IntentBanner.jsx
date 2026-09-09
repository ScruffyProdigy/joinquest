import {
  LAUNCH_GAME,
  LEAVE_GAME,
  LEAVE_TABLE_SEAT,
  bannerIntentPlayingHint,
  bannerIntentLaunchPendingHint,
  bannerTableBackfillHint,
  bannerTableSeatHint,
  bannerTableSeatLine,
} from '../../lib/playerCopy'
import {
  hasFormingTableIntent,
  hasReadyToPlayIntent,
  playingIntentTitle,
  resolveIntentLaunchUrl,
} from '../../lib/intent'
import { Button } from '../ui/button'

function LeaveError({ message }) {
  if (!message) {
    return null
  }
  return (
    <p className="intent-banner__error" role="alert">
      {message}
    </p>
  )
}

/**
 * Ready-to-play and forming-table intents. The queued state left this banner for a
 * page of its own in JQ-197, and a match that forms while the player is on that page
 * gets the launch step (JQ-136). This branch is what a player who is somewhere else
 * in the lobby sees, and the manual way in for anyone the countdown did not carry.
 */
export default function IntentBanner({
  activeIntent,
  activeTableSeat,
  busy,
  leaveError = null,
  onLeave,
}) {
  if (hasReadyToPlayIntent(activeIntent, activeTableSeat)) {
    const launchUrl = resolveIntentLaunchUrl(activeIntent, activeTableSeat)
    const title = playingIntentTitle(activeIntent, activeTableSeat)

    return (
      <aside className="intent-banner" role="region" aria-live="polite" aria-label="Your intent">
        <div className="intent-banner__copy">
          <p className="intent-banner__title">{title}</p>
          <p className="intent-banner__hint">
            {launchUrl ? bannerIntentPlayingHint() : bannerIntentLaunchPendingHint()}
          </p>
          <LeaveError message={leaveError} />
        </div>
        <div className="intent-banner__actions">
          {launchUrl ? (
            <Button asChild variant="default" size="sm">
              <a href={launchUrl}>{LAUNCH_GAME}</a>
            </Button>
          ) : null}
          <Button type="button" variant="secondary" size="sm" onClick={onLeave} disabled={busy}>
            {busy ? '…' : LEAVE_GAME}
          </Button>
        </div>
      </aside>
    )
  }

  if (!hasFormingTableIntent(activeIntent, activeTableSeat)) {
    return null
  }

  return (
    <aside className="intent-banner" role="region" aria-live="polite" aria-label="Your intent">
      <div className="intent-banner__copy">
        <p className="intent-banner__title">
          {bannerTableSeatLine(
            activeTableSeat.gameName,
            activeTableSeat.modeName,
            activeTableSeat.seatDisplayName,
          )}
        </p>
        <p className="intent-banner__hint">
          {activeTableSeat.backfillActive
            ? bannerTableBackfillHint(activeTableSeat.formingGaps)
            : bannerTableSeatHint()}
        </p>
        <LeaveError message={leaveError} />
      </div>
      <div className="intent-banner__actions">
        <Button type="button" variant="secondary" size="sm" onClick={onLeave} disabled={busy}>
          {busy ? '…' : LEAVE_TABLE_SEAT}
        </Button>
      </div>
    </aside>
  )
}
