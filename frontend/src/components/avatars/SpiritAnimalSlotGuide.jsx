import { JOURNEY_SLOTS } from '../../lib/spiritAnimalSlots'
import { cn } from '../../lib/utils'

export default function SpiritAnimalSlotGuide({ highlightKey = null, compact = false }) {
  if (compact) {
    return (
      <ul className="flex flex-wrap gap-2" role="list" aria-label="Journey slots">
        {JOURNEY_SLOTS.map((slot, index) => {
          const active = highlightKey === slot.key
          return (
            <li
              key={slot.key}
              className={cn(
                'flex items-center gap-1.5 rounded-full border border-border bg-muted/40 px-2.5 py-1',
                active && 'border-primary bg-primary/10',
              )}
              title={`${index + 1}. ${slot.name} — ${slot.prompt}`}
            >
              <img src={slot.imageUrl} alt="" className="size-5 rounded-full object-cover" />
              <span className="text-2xs font-medium text-foreground">{slot.name}</span>
            </li>
          )
        })}
      </ul>
    )
  }

  return (
    <ol className="flex flex-col gap-3" aria-label="Five chapters of the journey">
      {JOURNEY_SLOTS.map((slot, index) => {
        const active = highlightKey === slot.key
        return (
          <li
            key={slot.key}
            className={cn(
              'flex items-center gap-3 rounded-xl border border-border bg-muted/40 p-3',
              active && 'border-primary bg-primary/10',
            )}
          >
            <span className="font-mono-display text-xs text-muted-foreground" aria-hidden="true">
              {index + 1}
            </span>
            <img src={slot.imageUrl} alt="" className="size-8 shrink-0 rounded-full object-cover" />
            <div className="flex flex-col gap-0.5">
              <p className="text-sm font-semibold text-foreground">{slot.name}</p>
              <p className="text-xs text-muted-foreground">{slot.prompt}</p>
            </div>
          </li>
        )
      })}
    </ol>
  )
}
