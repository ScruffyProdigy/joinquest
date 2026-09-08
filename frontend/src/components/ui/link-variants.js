import { cva } from 'class-variance-authority'

/**
 * `pill` and `quiet` are lifted from the prototype exactly. `inline` is ours: the
 * prototype holds only two anchors -- a tutorial link coloured by the game's hue,
 * and a developer-facing npm link -- and neither suits a run of body copy.
 *
 * The brand colour is reserved for the option we actually want clicked, so only
 * `pill` carries it. An inline link is navigation, not a call to action, and takes
 * its colour from the text around it. Weight is what separates it instead -- which
 * a reader can see when colour is unavailable to them, and which is how the
 * prototype's own anchors do it (both are `font-bold`).
 *
 * Press states are opacity throughout, matching the prototype. Hover is our
 * addition: the prototype is mobile-first and specifies no hover state anywhere.
 *
 * `bg-transparent` is explicit rather than inherited: a button-rendered link
 * would otherwise pick up the user agent's control background.
 *
 * Every colour here is a token -- `text-primary`, `border-primary`,
 * `text-muted-foreground` -- so repainting the brand is a change to `--primary`
 * in tailwind.css and nothing in this file. Worth keeping that way: the
 * prototype already mixes teal with a magenta it uses for chrome, so the accent
 * is not settled.
 */
export const linkVariants = cva(
  'appearance-none border-0 p-0 [font-family:inherit] bg-transparent no-underline cursor-pointer transition-opacity outline-none focus-visible:ring-ring/50 focus-visible:ring-[3px] disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        inline: 'inline rounded-sm font-semibold text-foreground hover:opacity-80 active:opacity-70',
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
