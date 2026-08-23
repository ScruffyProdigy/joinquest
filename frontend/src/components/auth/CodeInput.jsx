import { cn } from '../../lib/utils'

const BOX_COUNT = 6
const BOX_INDICES = Array.from({ length: BOX_COUNT })

export default function CodeInput({ className, value = '', ref, ...props }) {
  return (
    <div className="relative h-11 w-full">
      <div className="pointer-events-none absolute inset-0 grid grid-cols-6 gap-2" aria-hidden="true">
        {BOX_INDICES.map((_, index) => (
          <div
            key={index}
            className={cn(
              'flex items-center justify-center rounded-md border border-border bg-input-background font-mono-display text-lg text-foreground',
              index < value.length && 'border-primary',
            )}
          >
            {value[index] ?? ''}
          </div>
        ))}
      </div>
      <input
        ref={ref}
        value={value}
        className={cn(
          'relative h-11 w-full rounded-md border-none bg-transparent text-center tracking-[0.5em] text-transparent caret-primary outline-none',
          className,
        )}
        {...props}
      />
    </div>
  )
}
