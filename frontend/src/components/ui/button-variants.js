import { cva } from 'class-variance-authority'

/**
 * The prototype has one button shape: a pill. Read from the deployed bundle, its
 * filled primary is `w-full py-4 rounded-[99px] font-bold text-[15px]` pressing
 * with `active:opacity-80` — the same geometry `Link`'s `pill` variant already
 * carries (JQ-72), so a button and a link-styled action beside it read as one
 * control at two weights rather than two different controls.
 *
 * Full width is the *default* size rather than an opt-in, because that is what the
 * prototype does: a button owns its row. Every call site that wanted the prototype's
 * shape was passing `w-full` by hand, which is the wrong way round.
 *
 * The compact sizes are the prototype's too — it pills its inline chips at
 * `px-[12px] py-[8px]` rather than squaring them off — so `sm` is a smaller pill,
 * not a different shape. There is no `lg`: the prototype has nothing above `py-4`,
 * and the old `lg` only existed to reach a size `default` now is.
 *
 * Sizing is padding, never a fixed height. `h-9` was what forced call sites into
 * `h-auto` to fit a chip or a wrapping label; with padding they simply fit.
 *
 * Press is opacity throughout, matching the prototype and `linkVariants`, except on
 * `card` — a surface that reads as liftable presses with `active:scale-[0.99]`,
 * again the prototype's own choice for its social sign-in row.
 *
 * Hover is ours. The prototype is mobile-first and specifies no hover state anywhere.
 *
 * Sizes use the type scale (`text-base` is 15px, `text-sm` is 13px) rather than the
 * raw pixel values the prototype ships, so restyling stays a change in tailwind.css.
 *
 * `OptionButton` deliberately stays a separate primitive. `card` is close to its
 * `row` geometry, but an option carries a tri-state `selected` that drives
 * `aria-pressed`, and Button has no business growing a pressed state to absorb it.
 */
export const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[99px] font-bold transition-all active:opacity-80 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0 outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 aria-invalid:border-destructive",
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        destructive:
          'bg-destructive text-white hover:bg-destructive/90 focus-visible:ring-destructive/20',
        outline:
          'border-[1.5px] border-border bg-background text-foreground hover:bg-accent hover:text-accent-foreground',
        secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
        ghost:
          'bg-transparent text-foreground border-[1.5px] border-transparent hover:bg-accent hover:text-accent-foreground',
        card: 'border border-border bg-card text-foreground shadow-sm hover:bg-accent/40 active:opacity-100 active:scale-[0.99]',
      },
      size: {
        default: 'w-full px-6 py-4 text-base',
        sm: 'px-3 py-2 text-sm',
        // A chip that leads with an avatar: the left padding is the avatar's own
        // ring, so padding it again would float the image off the edge.
        chip: 'py-1 pr-3 pl-1 text-sm',
        icon: 'size-9 p-0',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)
