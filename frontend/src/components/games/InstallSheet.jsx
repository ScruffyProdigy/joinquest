import {
  INSTALL_BODY,
  INSTALL_DISMISS,
  INSTALL_STEP_ADD,
  INSTALL_STEP_ADD_WHERE,
  INSTALL_STEP_SHARE,
  INSTALL_STEP_SHARE_WHERE,
  INSTALL_TITLE,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'
import { Sheet, SheetContent, SheetTitle } from '../ui/sheet'

/* iOS Safari's Share glyph, drawn at toolbar size. The hard part of this
   instruction is recognising the button, not reading its name. */
function ShareGlyph() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="M12 3.5v11"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
      />
      <path
        d="M8.4 7.1 12 3.5l3.6 3.6"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M7 10.5H5.6A1.6 1.6 0 0 0 4 12.1v6.8a1.6 1.6 0 0 0 1.6 1.6h12.8a1.6 1.6 0 0 0 1.6-1.6v-6.8a1.6 1.6 0 0 0-1.6-1.6H17"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
      />
    </svg>
  )
}

/* The Add to Home Screen row's own glyph, so step two is recognisable too. */
function AddToHomeGlyph() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <rect
        x="4"
        y="4"
        width="16"
        height="16"
        rx="4.5"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
      />
      <path
        d="M12 8.6v6.8M8.6 12h6.8"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
      />
    </svg>
  )
}

function Step({ number, glyph, action, where }) {
  return (
    <li className="install-sheet__step">
      <span className="install-sheet__glyph">{glyph}</span>
      <span className="install-sheet__step-text">
        <span className="install-sheet__step-action">
          <span className="install-sheet__step-number" aria-hidden="true">
            {number}
          </span>
          {action}
        </span>
        <span className="install-sheet__step-where">{where}</span>
      </span>
    </li>
  )
}

/**
 * Teaches the Share-sheet gesture. iOS gives no way to trigger the install, so
 * showing the player where to look is the only route to notifications there.
 */
export default function InstallSheet({ open, onOpenChange, onDismiss }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="bottom" aria-label={INSTALL_TITLE}>
        <SheetTitle>{INSTALL_TITLE}</SheetTitle>
        <p className="install-sheet__body">{INSTALL_BODY}</p>

        <ol className="install-sheet__steps">
          <Step
            number="1"
            glyph={<ShareGlyph />}
            action={INSTALL_STEP_SHARE}
            where={INSTALL_STEP_SHARE_WHERE}
          />
          <Step
            number="2"
            glyph={<AddToHomeGlyph />}
            action={INSTALL_STEP_ADD}
            where={INSTALL_STEP_ADD_WHERE}
          />
        </ol>

        <Button type="button" variant="secondary" onClick={onDismiss}>
          {INSTALL_DISMISS}
        </Button>
      </SheetContent>
    </Sheet>
  )
}
