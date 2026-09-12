import { cn } from '../../lib/utils'

function Input({ className, type = 'text', ref, ...props }) {
  return (
    <input
      ref={ref}
      type={type}
      data-slot="input"
      className={cn(
        // `box-border` for the same reason Dialog carries it: Tailwind loads
        // without preflight here, so `w-full` on a content-box element comes out
        // its padding and border wider than its container — 26px of overflow on
        // every field, which shows the moment one is asked for full-bleed.
        'flex h-9 w-full min-w-0 box-border rounded-md border border-border bg-input-background px-3 py-1 text-base text-foreground shadow-xs transition-colors outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
