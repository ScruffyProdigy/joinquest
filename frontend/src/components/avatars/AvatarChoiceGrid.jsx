import { CheckIcon } from 'lucide-react'
import { OptionButton } from '../ui/option-button'

/**
 * Every screen that offers a player a face to pick draws it here: a row per
 * choice, an avatar beside the name that goes with it, two up.
 *
 * Callers hand over `choices` — `{ key, avatarUrl, displayName }` — and keep
 * their own idea of what a pick means. What varies between them is only which
 * name the row carries: the generated handle a visitor is choosing, the colour
 * and animal of a face offered to someone already named, or a journey icon's
 * own name.
 *
 * It is one component on purpose. Drawn three separate times, these drifted:
 * the guest picker stayed two up, the face-only prompt went six across, and
 * the profile editor never left a single column — so the same act of picking
 * a face looked like three different screens depending on where you met it.
 *
 * Two up at every width, which is the prototype's grid. It used to go one up
 * below `sm`, and six stacked rows are what pushed the first-entry gate past
 * the bottom of a phone screen: the same six choices are half as tall in two
 * columns, and that is the difference between fitting and being cut off.
 *
 * `selectedKey` is deliberately optional. Left off, a row is a plain action
 * and announces itself as one; passed, every row becomes a toggle, which is
 * what an editor showing a current choice needs — and what the prototype's
 * pick-then-confirm gate needs too. A selected row carries the prototype's
 * check badge, so the choice is not resting on the border colour alone.
 */
export default function AvatarChoiceGrid({ choices, selectedKey, busyKey, busyLabel = 'Starting…', disabled = false, onPick }) {
  return (
    <ul className="m-0 grid list-none grid-cols-2 gap-2 p-0" role="list">
      {choices.map((choice) => {
        const selected = selectedKey === undefined ? undefined : choice.key === selectedKey
        return (
          <li key={choice.key}>
            <OptionButton
              variant="choice"
              selected={selected}
              disabled={disabled || Boolean(busyKey)}
              aria-label={selected ? `${choice.displayName} (selected)` : choice.displayName}
              onClick={() => onPick(choice)}
            >
              {/* The check rides the avatar rather than sitting at the end of the
                  row, which is where the prototype puts it. Two up on a phone, the
                  name has about thirteen characters of room and ours are generated
                  up to twenty long; a badge on the row costs a quarter of that and
                  pushes a third line onto half the names. On the disc it costs
                  nothing, and it cannot reflow the name under the finger that just
                  picked it. */}
              <span className="relative shrink-0">
                <img src={choice.avatarUrl} alt="" className="size-7 rounded-full object-cover" />
                {selected ? (
                  <span
                    aria-hidden="true"
                    className="absolute -right-0.5 -bottom-0.5 flex size-3.5 items-center justify-center rounded-full bg-primary ring-2 ring-background"
                  >
                    <CheckIcon className="size-2 text-primary-foreground" strokeWidth={4} />
                  </span>
                ) : null}
              </span>
              <span className="min-w-0 flex-1 font-mono-display text-sm break-words text-foreground">
                {busyKey === choice.key ? busyLabel : choice.displayName}
              </span>
            </OptionButton>
          </li>
        )
      })}
    </ul>
  )
}
