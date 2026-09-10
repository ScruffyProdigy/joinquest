import {
  LEAVE_GAME,
  REJOIN_MATCH,
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

function BannerError({ message }) {
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
  rejoinError = null,
  rejoining = false,
  onLeave,
  onRejoin,
}) {
  if (hasReadyToPlayIntent(activeIntent, activeTableSeat)) {
    // The launch URL only tells us the match is provisioned and has somewhere to
    // send the player; the URL the player actually travels on is minted on click,
    // because a rejoin token is short-lived by design (JQ-86).
    const ready = Boolean(resolveIntentLaunchUrl(activeIntent, activeTableSeat))
    const title = playingIntentTitle(activeIntent, activeTableSeat)

    return (
      <aside className="intent-banner" role="region" aria-live="polite" aria-label="Your intent">
        <div className="intent-banner__copy">
          <p className="intent-banner__title">{title}</p>
          <p className="intent-banner__hint">
            {ready ? bannerIntentPlayingHint() : bannerIntentLaunchPendingHint()}
          </p>
          <BannerError message={rejoinError} />
          <BannerError message={leaveError} />
        </div>
        <div className="intent-banner__actions">
          {ready ? (
            <Button
              type="button"
              variant="default"
              size="sm"
              onClick={onRejoin}
              disabled={rejoining || busy}
            >
              {rejoining ? '…' : REJOIN_MATCH}
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
        <BannerError message={leaveError} />
      </div>
      <div className="intent-banner__actions">
        <Button type="button" variant="secondary" size="sm" onClick={onLeave} disabled={busy}>
          {busy ? '…' : LEAVE_TABLE_SEAT}
        </Button>
      </div>
    </aside>
  )
}
