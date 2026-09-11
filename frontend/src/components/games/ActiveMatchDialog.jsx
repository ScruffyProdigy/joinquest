import { hasReadyToPlayIntent } from '../../lib/intent'
import { MATCH_DIALOG_TITLE } from '../../lib/playerCopy'
import { Dialog, DialogContent, DialogTitle } from '../ui/dialog'
import LaunchStep from './LaunchStep'

/**
 * What a player with a live match sees when they turn up in the lobby (JQ-261).
 *
 * This replaces the intent banner, and inverts its premise. The banner was a strip
 * you could ignore, mounted on the two pages that happened to render it; a player
 * who lost their tab and landed anywhere else had no route back at all (JQ-86).
 * This is a modal instead: it covers whatever page it appears over, and it does not
 * take dismissal for an answer. There is nothing to do in the lobby while a game we
 * sent you to is still running, so the only two ways out are the two that are true
 * -- go back in, or leave the game.
 *
 * The body is `LaunchStep`, the same component the waiting page and the group screen
 * use for a match that has just formed. Reusing it is what keeps every entrance into
 * a game the same shape; `rejoin` is the only thing that differs, and it is what
 * makes the way in mint on click rather than travel on a token that went stale while
 * the player was away.
 */
export default function ActiveMatchDialog({
  activeIntent,
  activeTableSeat,
  busy,
  leaveError,
  rejoinError,
  rejoining,
  onLeave,
  onRejoin,
}) {
  if (!hasReadyToPlayIntent(activeIntent, activeTableSeat)) {
    return null
  }

  const refuseToClose = (event) => event.preventDefault()

  return (
    // No `onOpenChange`: there is no close state to move to. Escape and a click on
    // the overlay are refused for the same reason the close button is absent --
    // dismissing this would put the player back where the banner left them, in a
    // lobby with a live match and no way into it.
    <Dialog open>
      <DialogContent
        showCloseButton={false}
        aria-describedby={undefined}
        onEscapeKeyDown={refuseToClose}
        onPointerDownOutside={refuseToClose}
        onInteractOutside={refuseToClose}
      >
        {/* The visible heading belongs to LaunchStep, which owns the whole body and
            changes it with the state. This names the dialog for assistive tech
            without a second heading competing with it on screen. */}
        <DialogTitle className="sr-only">{MATCH_DIALOG_TITLE}</DialogTitle>

        <LaunchStep
          rejoin
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          busy={busy}
          leaveError={leaveError}
          rejoinError={rejoinError}
          rejoining={rejoining}
          onLeave={onLeave}
          onRejoin={onRejoin}
        />
      </DialogContent>
    </Dialog>
  )
}
