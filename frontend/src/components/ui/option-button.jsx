import { cn } from '../../lib/utils'
import { optionButtonVariants } from './option-button-variants'

/**
 * `selected` is deliberately tri-state. Left undefined the control is a plain
 * action and gets no `aria-pressed` at all; passed either way it becomes a toggle
 * and announces itself as one, so selection never rests on colour alone.
 */
function OptionButton({ className, variant, selected, ...props }) {
  return (
    <button
      data-slot="option-button"
      type="button"
      aria-pressed={selected === undefined ? undefined : selected}
      className={cn(optionButtonVariants({ variant, selected: Boolean(selected), className }))}
      {...props}
    />
  )
}

export { OptionButton }
