import { accentColorFor } from '../../lib/gameAccent'
import { regroupActionsFor } from '../../lib/regroup'
import {
  formatBackToGame,
  REGROUP_ANOTHER_ROUND,
  REGROUP_BACK_TO_TABLE,
  REGROUP_CHOOSE_AGAIN,
  REGROUP_FIND_SOMETHING_NEW,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'

/**
 * The ways off the return screen, pinned to the bottom of it.
 *
 * The prototype puts these in a bar of their own rather than inside the roster card, and the
 * reason shows up the moment the roster is absent: a solo player, or anyone who exited early,
 * has no card for the buttons to live in. A bar belongs to the screen, so it is there in
 * every state — including the one where the match is still running and the only honest action
 * is to leave (JQ-277).
 *
 * `sticky` rather than the prototype's `fixed`: production already has this pattern in
 * `GroupStartBar`, and sticky keeps the bar in flow so it cannot cover the end of the
 * standings on a short viewport.
 */
export default function MatchActionBar({ children }) {
  return (
    <div className="sticky bottom-0 z-30 border-t border-border bg-background/95 px-4 pt-3 pb-4 backdrop-blur-md">
      <div className="mx-auto flex w-full max-w-lg flex-col gap-2">{children}</div>
    </div>
  )
}

/**
 * The finished-match set: another round, a way back into the game, and a way out.
 *
 * The primary carries the game's own accent gradient, as the prototype's does. That is the
 * one place on this screen the game gets to be itself, and `accentColorFor` is where every
 * other surface in the app reads the same colour from.
 */
export function RegroupActions({ result, viewerId, busy, onPlayAgain, onChooseAgain, onBackToGame, onDecline }) {
  const game = result?.game
  const { viewerIn, showChooseAgain, showBackToGame } = regroupActionsFor(result, viewerId)
  const accent = accentColorFor(game?.slug || game?.id || 'game', game?.accentColor)

  return (
    <>
      <Button
        type="button"
        disabled={busy}
        onClick={onPlayAgain}
        // The gradient replaces the variant's own background, so the token-driven
        // foreground stays: both accent stops are dark enough for it by construction.
        style={{ background: accent.badge }}
      >
        {viewerIn ? REGROUP_BACK_TO_TABLE : REGROUP_ANOTHER_ROUND}
      </Button>
      {showChooseAgain ? (
        <Button type="button" variant="secondary" disabled={busy} onClick={onChooseAgain}>
          {REGROUP_CHOOSE_AGAIN}
        </Button>
      ) : null}
      {showBackToGame ? (
        <Button type="button" variant="secondary" disabled={busy} onClick={onBackToGame}>
          {formatBackToGame(game?.name)}
        </Button>
      ) : null}
      <Button type="button" variant="ghost" disabled={busy} onClick={onDecline}>
        {REGROUP_FIND_SOMETHING_NEW}
      </Button>
    </>
  )
}
