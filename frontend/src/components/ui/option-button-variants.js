import { cva } from 'class-variance-authority'

/**
 * The pickable option: an avatar, a display name, a quiz answer. Three call sites
 * had byte-identical copies of the row treatment and a fourth had the tile, so it
 * lives here once instead.
 *
 * Not a Button. Button is a fixed-height inline control with `whitespace-nowrap`;
 * these are full-width surfaces that hold an image and wrapping text, and forcing
 * them through Button meant four override classes each to undo it.
 *
 * Hover previews the selected treatment rather than merely greying, so the two
 * states read as the same gesture at different strengths.
 */
export const optionButtonVariants = cva(
  'flex w-full cursor-pointer appearance-none border border-border bg-muted/40 [font-family:inherit] transition-colors hover:border-primary hover:bg-primary/10 disabled:pointer-events-none disabled:opacity-60 outline-none focus-visible:ring-ring/50 focus-visible:ring-[3px]',
  {
    variants: {
      variant: {
        row: 'items-center gap-3 rounded-full px-4 py-2.5 text-left',
        tile: 'flex-col items-center gap-1 rounded-2xl px-2 py-3',
      },
      selected: {
        true: 'border-primary bg-primary/10',
        false: '',
      },
    },
    defaultVariants: {
      variant: 'row',
      selected: false,
    },
  },
)
