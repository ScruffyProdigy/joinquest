import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Link } from '../ui/link'
import { listAccentColors } from '../../lib/gameAccent'
import ComponentLibrarySection from './ComponentLibrarySection'

const TYPE_SCALE = [
  { className: 'text-2xs', label: '2xs / 11px — micro meta (counts, timestamps)' },
  { className: 'text-xs', label: 'xs / 12px — badges, pills, small buttons' },
  { className: 'text-sm', label: 'sm / 13px — secondary/supporting text' },
  { className: 'text-base', label: 'base / 15px — default body/UI text, card titles' },
  { className: 'text-lg', label: 'lg / 18px — subheadings, modal titles' },
  { className: 'text-xl', label: 'xl / 24px — section headings' },
  { className: 'text-2xl', label: '2xl / 34px — hero heading, mobile' },
  { className: 'text-3xl', label: '3xl / 40px — hero heading, desktop' },
]

const SPACING_STEPS = [1, 2, 3, 4, 6, 8, 10, 12, 16]

export default function StylePreviewPage() {
  return (
    <main className="min-h-screen bg-background text-foreground p-10 flex flex-col gap-8">
      <div>
        <h1 className="font-heading text-3xl font-bold">Design tokens</h1>
        <p className="font-sans text-muted-foreground mt-2">
          Live reference for the dark-first theme: colors, type scale, spacing, and per-game accent
          colors, alongside a Tailwind v4 + shadcn/ui pipeline check.
        </p>
      </div>

      <section className="flex flex-col gap-4">
        <h2 className="font-heading text-xl font-semibold">Buttons</h2>
        <p className="text-sm text-muted-foreground">
          One shape, taken from the prototype: a <code>rounded-[99px]</code> pill that owns its
          row. Full width is the default size, not something a call site asks for. Sizing is
          padding — there is no fixed height, and nothing above <code>py-4</code>.
        </p>
        <div className="flex max-w-xs flex-col gap-2">
          <Button>Default</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="outline">Outline</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="destructive">Destructive</Button>
          <Button variant="card">Card — presses with a scale</Button>
        </div>
        <p className="text-sm text-muted-foreground">
          The compact sizes are the prototype&rsquo;s own inline chips — smaller pills, not a
          different shape.
        </p>
        <div className="flex flex-wrap items-center gap-3">
          <Button size="sm">Small</Button>
          <Button variant="secondary" size="sm">
            Small secondary
          </Button>
          <Button variant="ghost" size="chip">
            Chip
          </Button>
          <Button variant="ghost" size="icon" aria-label="Close">
            ✕
          </Button>
        </div>
      </section>

      <section className="flex flex-col gap-4">
        <h2 className="font-heading text-xl font-semibold">Button and Link, side by side</h2>
        <p className="text-sm text-muted-foreground">
          The same geometry at different weights (JQ-223). A filled button and a pill link stacked
          together should read as one control family — same radius, same vertical padding, same
          15px bold label — with only the fill telling them apart.
        </p>
        <div className="flex max-w-xs flex-col gap-2">
          <Button>Filled button</Button>
          <Link variant="pill" href="#buttons">
            Outlined pill link
          </Link>
          <Link variant="quiet">Quiet pill link</Link>
        </div>
      </section>

      <section className="flex flex-col gap-4">
        <h2 className="font-heading text-xl font-semibold">Links</h2>
        <p className="text-sm text-muted-foreground">
          Nothing here underlines, in any state. <code>href</code> picks the element: an anchor
          for navigation, a button for an action that only runs a handler. Only the pill carries
          the brand colour — it is the option we want clicked. An inline link is navigation, so it
          takes its own <code>--link</code> colour and its weight, never the brand's.
        </p>
        <p className="text-sm">
          An <Link href="#links">Inline link</Link> sits inside a sentence.
        </p>
        <div className="flex max-w-xs flex-col gap-2">
          <Link variant="pill" href="#links">
            Pill link
          </Link>
          <Link variant="quiet">Quiet action</Link>
        </div>
      </section>

      <section className="flex flex-col gap-2">
        <h2 className="font-heading text-xl font-semibold">Fonts</h2>
        <p className="font-heading text-2xl">Cal Sans (heading)</p>
        <p className="font-sans text-lg">Plus Jakarta Sans (body)</p>
        <p className="font-mono-display text-sm">DM Mono (mono display)</p>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-heading text-xl font-semibold">Type scale</h2>
        <div className="flex flex-col gap-2">
          {TYPE_SCALE.map((step) => (
            <div key={step.className} className="flex items-baseline gap-4">
              <span className={`${step.className} font-heading w-16 shrink-0`}>Aa</span>
              <span className="font-mono-display text-2xs text-muted-foreground">{step.label}</span>
            </div>
          ))}
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-heading text-xl font-semibold">Spacing</h2>
        <p className="font-sans text-sm text-muted-foreground">
          Spacing intentionally uses Tailwind's default 4px-based scale — not left to accident, just not
          worth reinventing.
        </p>
        <div className="flex flex-col gap-2">
          {SPACING_STEPS.map((step) => (
            <div key={step} className="flex items-center gap-4">
              <div
                className="bg-primary rounded-sm h-3"
                style={{ width: `${step * 0.25}rem` }}
              />
              <span className="font-mono-display text-2xs text-muted-foreground">
                {step} / {step * 4}px
              </span>
            </div>
          ))}
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-heading text-xl font-semibold">Outcome colours</h2>
        <p className="font-sans text-sm text-muted-foreground">
          <code className="font-mono-display text-2xs">--success</code> and{' '}
          <code className="font-mono-display text-2xs">--warning</code> report how something went
          — a winner, a player who has opted back in, a match still running. Each is used as text
          on its own surface, so both pairings are contrast-checked in{' '}
          <code className="font-mono-display text-2xs">src/theme.test.js</code>.
        </p>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { name: 'success', color: 'var(--success)', surface: 'var(--success-surface)' },
            { name: 'warning', color: 'var(--warning)', surface: 'var(--warning-surface)' },
          ].map((token) => (
            <div
              key={token.name}
              className="flex flex-col gap-2 rounded-2xl p-3"
              style={{ backgroundColor: token.surface }}
            >
              <span className="font-sans text-xs font-bold capitalize" style={{ color: token.color }}>
                {token.name}
              </span>
              <span className="font-sans text-2xs text-foreground">on its surface</span>
            </div>
          ))}
          {['success', 'warning'].map((name) => (
            <Badge key={name} variant={name} className="self-start">
              {name}
            </Badge>
          ))}
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-heading text-xl font-semibold">Per-game accents</h2>
        <p className="font-sans text-sm text-muted-foreground">
          Deterministic per-game accent, hashed from the game's slug — see{' '}
          <code className="font-mono-display text-2xs">accentColorFor</code> in{' '}
          <code className="font-mono-display text-2xs">src/lib/gameAccent.js</code>.
        </p>
        <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
          {listAccentColors().map((accent) => (
            <div
              key={accent.name}
              className="rounded-2xl overflow-hidden shadow-sm"
              style={{ background: accent.cardBg, border: `1px solid ${accent.border}` }}
            >
              <div className="h-10" style={{ background: accent.badge }} />
              <div className="p-2">
                <span className="font-sans text-xs font-semibold capitalize">{accent.name}</span>
              </div>
            </div>
          ))}
        </div>
      </section>

      <ComponentLibrarySection />
    </main>
  )
}
