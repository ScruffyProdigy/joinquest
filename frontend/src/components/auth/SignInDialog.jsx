import { useRef } from 'react'
import { SIGN_IN_DIALOG_TITLE, SIGN_IN_MAYBE_LATER } from '../../lib/playerCopy'
import { Dialog, DialogContent, DialogTitle } from '../ui/dialog'
import { Link } from '../ui/link'
import SignInBenefits from './SignInBenefits'
import SignInPanel from './SignInPanel'

/**
 * Sign-in as a modal, for anywhere a page needs to offer it without navigating.
 * The guest option is left out on purpose: anyone who can open this already has
 * a guest session from the first-entry gate, so "Play as guest" would do nothing.
 *
 * The rotating benefit cards replace the one static hint this used to carry
 * (JQ-249), so the dialog describes itself through them — hence no
 * `DialogDescription`, and `aria-describedby` opted out rather than left
 * dangling at an element that no longer exists.
 */
export default function SignInDialog({ open, onOpenChange }) {
  const contentRef = useRef(null)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        ref={contentRef}
        className="gap-4"
        aria-describedby={undefined}
        onOpenAutoFocus={(event) => {
          // Left alone, Radix focuses the first tabbable child -- which is now a
          // carousel dot. That announces "Reason 1 of 4" instead of the dialog,
          // and parks focus inside the carousel, which pauses its rotation before
          // it has run once. The dialog itself is the right landing place.
          event.preventDefault()
          contentRef.current?.focus()
        }}
      >
        {/* pr-8 keeps the title clear of the dialog's close button in the corner. */}
        <DialogTitle className="font-heading text-xl font-semibold pr-8">
          {SIGN_IN_DIALOG_TITLE}
        </DialogTitle>

        <SignInBenefits />

        <SignInPanel heading={null} showGuestOption={false} />

        <Link variant="quiet" className="self-center" onClick={() => onOpenChange(false)}>
          {SIGN_IN_MAYBE_LATER}
        </Link>
      </DialogContent>
    </Dialog>
  )
}
