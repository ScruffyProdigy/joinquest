import { SIGN_IN_DIALOG_HINT, SIGN_IN_DIALOG_TITLE } from '../../lib/playerCopy'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../ui/dialog'
import SignInPanel from './SignInPanel'

/**
 * Sign-in as a modal, for anywhere a page needs to offer it without navigating.
 * The guest option is left out on purpose: anyone who can open this already has
 * a guest session from the first-entry gate, so "Jump in" would do nothing.
 */
export default function SignInDialog({ open, onOpenChange }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-4">
        <div className="flex flex-col gap-1">
          <DialogTitle className="font-heading text-xl font-semibold">{SIGN_IN_DIALOG_TITLE}</DialogTitle>
          <DialogDescription>{SIGN_IN_DIALOG_HINT}</DialogDescription>
        </div>
        <SignInPanel heading={null} showGuestOption={false} />
      </DialogContent>
    </Dialog>
  )
}
