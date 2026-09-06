import {
  LAUNCH_GAME,
  LEAVE_GAME,
  LEAVE_TABLE_SEAT,
  STOP_LOOKING,
  bannerIntentPlayingHint,
  bannerIntentLaunchPendingHint,
  bannerIntentWaitingHint,
  bannerLiveUpdatesPausedHint,
  bannerTableBackfillHint,
  bannerTableSeatHint,
  bannerTableSeatLine,
  bannerWaitingLine,
} from '../../lib/playerCopy'
import {
  hasFormingTableIntent,
  hasReadyToPlayIntent,
  hasWaitingIntent,
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

export default function IntentBanner({
  activeIntent,
  activeTableSeat,
  busy,
  liveUpdatesConnected = true,
  leaveError = null,
  onLeave,
}) {
  if (hasWaitingIntent(activeIntent)) {
    return (
      <aside className="intent-banner" role="region" aria-live="polite" aria-label="Your intent">
        <div className="intent-banner__copy">
          <p className="intent-banner__title">
            {bannerWaitingLine(
              activeIntent.gameName,
              activeIntent.queuedCount,
              activeIntent.queuePathDisplayName,
              activeIntent.formingGaps,
            )}
          </p>
          <p className="intent-banner__hint">
            {liveUpdatesConnected ? bannerIntentWaitingHint() : bannerLiveUpdatesPausedHint()}
          </p>
          <LeaveError message={leaveError} />
        </div>
        <div className="intent-banner__actions">
          <Button type="button" variant="default" onClick={onLeave} disabled={busy}>
            {busy ? '…' : STOP_LOOKING}
          </Button>
        </div>
      </aside>
    )
  }

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
            <Button asChild variant="default" className="intent-banner__cta font-semibold">
              <a href={launchUrl}>{LAUNCH_GAME}</a>
            </Button>
          ) : null}
          <Button
            type="button"
            variant="secondary"
            onClick={onLeave}
            disabled={busy}
          >
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
        <Button type="button" variant="secondary" onClick={onLeave} disabled={busy}>
          {busy ? '…' : LEAVE_TABLE_SEAT}
        </Button>
      </div>
    </aside>
  )
}
