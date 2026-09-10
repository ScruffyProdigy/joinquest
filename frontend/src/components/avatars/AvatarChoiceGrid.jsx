import { OptionButton } from '../ui/option-button'

/**
 * Every screen that offers a player a face to pick draws it here: a row per
 * choice, an avatar beside the name that goes with it, two up once there is
 * room for it.
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
 * `selectedKey` is deliberately optional. Left off, a row is a plain action
 * and announces itself as one; passed, every row becomes a toggle, which is
 * what an editor showing a current choice needs.
 */
export default function AvatarChoiceGrid({ choices, selectedKey, busyKey, busyLabel = 'Starting…', disabled = false, onPick }) {
  return (
    <ul className="m-0 grid list-none gap-2 p-0 sm:grid-cols-2" role="list">
      {choices.map((choice) => {
        const selected = selectedKey === undefined ? undefined : choice.key === selectedKey
        return (
          <li key={choice.key}>
            <OptionButton
              selected={selected}
              disabled={disabled || Boolean(busyKey)}
              aria-label={selected ? `${choice.displayName} (selected)` : choice.displayName}
              onClick={() => onPick(choice)}
            >
              <img src={choice.avatarUrl} alt="" className="size-8 shrink-0 rounded-full object-cover" />
              <span className="font-mono-display text-sm text-foreground">
                {busyKey === choice.key ? busyLabel : choice.displayName}
              </span>
            </OptionButton>
          </li>
        )
      })}
    </ul>
  )
}
