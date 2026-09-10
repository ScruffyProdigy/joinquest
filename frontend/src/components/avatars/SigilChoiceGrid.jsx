import { OptionButton } from '../ui/option-button'

/**
 * The six sigils, drawn the same way wherever they are offered — one row per
 * face, two up once there is room for it.
 *
 * Both prompts that hand out a face share this. They differ only in what the
 * row says: the guest picker offers the generated handle, because the name is
 * part of what is being picked, and the avatar prompt offers the colour and
 * animal alone, because the player already chose what to be called.
 *
 * It is one component on purpose. When the avatar prompt drew its own grid it
 * drifted to a cramped six-across row while the guest picker stayed two-up, so
 * the same six faces read as two different screens depending on which half of
 * an identity happened to be missing.
 */
export default function SigilChoiceGrid({ identities, pendingKey, textOf, labelOf, onPick }) {
  return (
    <ul className="m-0 grid list-none gap-2 p-0 sm:grid-cols-2" role="list">
      {identities.map((identity) => (
        <li key={identity.avatarKey}>
          <OptionButton
            disabled={Boolean(pendingKey)}
            aria-label={labelOf?.(identity)}
            onClick={() => void onPick(identity)}
          >
            <img src={identity.imageUrl} alt="" className="size-8 shrink-0 rounded-full" />
            <span className="font-mono-display text-sm text-foreground">
              {pendingKey === identity.avatarKey ? 'Starting…' : textOf(identity)}
            </span>
          </OptionButton>
        </li>
      ))}
    </ul>
  )
}
