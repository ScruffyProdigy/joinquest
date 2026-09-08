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
        <div className="flex flex-wrap gap-3">
          <Button>Default</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="outline">Outline</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="destructive">Destructive</Button>
        </div>
      </section>

      <section className="flex flex-col gap-4">
        <h2 className="font-heading text-xl font-semibold">Links</h2>
        <p className="text-sm text-muted-foreground">
          Nothing here underlines, in any state. <code>href</code> picks the element: an anchor
          for navigation, a button for an action that only runs a handler.
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
