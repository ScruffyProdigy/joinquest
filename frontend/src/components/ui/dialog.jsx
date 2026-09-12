import * as DialogPrimitive from '@radix-ui/react-dialog'
import { XIcon } from 'lucide-react'

import { cn } from '../../lib/utils'

function Dialog(props) {
  return <DialogPrimitive.Root data-slot="dialog" {...props} />
}

function DialogTrigger(props) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />
}

function DialogPortal(props) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />
}

function DialogClose(props) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />
}

function DialogOverlay({ className, ...props }) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        'data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-black/50',
        className,
      )}
      {...props}
    />
  )
}

/**
 * The prototype's other dialog shape: not a card floating over the page but a
 * takeover that owns the whole screen, sliding up from the bottom edge. It is a
 * variant rather than a second component because everything else about it is a
 * dialog — the same portal, scrim, focus trap and escape handling.
 *
 * A phone is the case it is drawn for, and the geometry comes from the prototype:
 * a top inset that the safe area can push down but not pull up, and blocks spaced
 * by `justify-between` rather than stacked from the top. It scrolls when it has
 * to; the prototype pins its content with `touch-none`, which reads fine at one
 * screen size and clips the bottom off every shorter one.
 *
 * The inset is 56px against the prototype's 100 because `justify-between` already
 * spreads the blocks on a screen with room to spare — the number only decides how
 * close the heading may come to the notch on one without.
 *
 * Its blocks are capped and centred rather than left to fill the width. The
 * prototype is drawn at phone width, where edge-to-edge means 335px; on a desktop
 * the same classes would give a 1200px-wide button.
 */
const TAKEOVER_CLASSES =
  'inset-0 flex max-w-none translate-x-0 translate-y-0 flex-col items-start justify-between gap-0 overflow-y-auto rounded-none border-0 bg-background p-0 px-5 shadow-none [&>*]:mx-auto [&>*]:w-full [&>*]:max-w-sm pt-[max(env(safe-area-inset-top,0px),56px)] pb-[calc(env(safe-area-inset-bottom,0px)+24px)] data-[state=closed]:zoom-out-100 data-[state=open]:zoom-in-100 data-[state=closed]:slide-out-to-bottom data-[state=open]:slide-in-from-bottom sm:max-w-none'

function DialogContent({ className, children, showCloseButton = true, takeover = false, ...props }) {
  return (
    <DialogPortal>
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        data-takeover={takeover ? '' : undefined}
        className={cn(
          // `box-border` is not decoration: Tailwind loads here without preflight
          // (see tailwind.css), so the default is `content-box` and every width
          // set here — `w-full`, the max-widths, the takeover's `inset-0` — came
          // out 48px wider than asked for, padding included. On a phone that is a
          // dialog wider than the screen, which is what cut the first-entry gate
          // off at the right edge.
          'box-border bg-card text-card-foreground data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 fixed top-[50%] left-[50%] z-50 grid w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-lg border p-6 shadow-lg duration-200 sm:max-w-lg',
          takeover && TAKEOVER_CLASSES,
          className,
        )}
        {...props}
      >
        {children}
        {showCloseButton ? (
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className="ring-offset-background focus:ring-ring data-[state=open]:bg-accent data-[state=open]:text-muted-foreground absolute top-4 right-4 rounded-xs opacity-70 transition-opacity hover:opacity-100 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:pointer-events-none"
          >
            <XIcon className="size-4" />
            <span className="sr-only">Close</span>
          </DialogPrimitive.Close>
        ) : null}
      </DialogPrimitive.Content>
    </DialogPortal>
  )
}

function DialogHeader({ className, ...props }) {
  return <div data-slot="dialog-header" className={cn('flex flex-col gap-2 text-center sm:text-left', className)} {...props} />
}

function DialogFooter({ className, ...props }) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn('flex flex-col-reverse gap-2 sm:flex-row sm:justify-end', className)}
      {...props}
    />
  )
}

function DialogTitle({ className, ...props }) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn('text-lg leading-none font-semibold', className)}
      {...props}
    />
  )
}

function DialogDescription({ className, ...props }) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn('text-muted-foreground text-sm', className)}
      {...props}
    />
  )
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
}
