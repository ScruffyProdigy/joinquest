import { Sheet, SheetContent, SheetTitle } from '../ui/sheet'

export default function RoomSheet({ open, onDismiss, children }) {
  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          onDismiss()
        }
      }}
    >
      <SheetContent side="bottom" className="h-[90vh] gap-0 p-0" aria-label="Room chat">
        <SheetTitle className="sr-only">Room chat</SheetTitle>
        <div className="flex h-full flex-col overflow-hidden">{children}</div>
      </SheetContent>
    </Sheet>
  )
}
