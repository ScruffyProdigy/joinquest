import { cva } from 'class-variance-authority'

/**
 * The prototype has no underlined text anywhere: every link-shaped control is a
 * full-width pill or a plain text action, and press states are opacity rather
 * than decoration. `pill` and `quiet` are lifted from it directly. `inline` is
 * our own -- the prototype never puts a link inside a sentence, and our legal
 * pages and developer docs do -- so it leans on the brand colour to stay
 * distinguishable from body copy without reaching for an underline.
 *
 * `bg-transparent` is explicit rather than inherited: a button-rendered link
 * would otherwise pick up the user agent's control background.
 */
export const linkVariants = cva(
  'appearance-none border-0 p-0 [font-family:inherit] bg-transparent no-underline cursor-pointer transition-opacity outline-none focus-visible:ring-ring/50 focus-visible:ring-[3px] disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        inline: 'inline rounded-sm text-primary hover:opacity-80 active:opacity-70',
        pill: 'flex w-full items-center justify-center gap-2 rounded-[99px] border-[1.5px] border-primary py-4 text-[15px] font-bold text-primary active:opacity-80',
        quiet:
          'block w-full rounded-[99px] py-3.5 text-center text-[14px] font-bold text-muted-foreground active:opacity-60',
      },
    },
    defaultVariants: {
      variant: 'inline',
    },
  },
)
