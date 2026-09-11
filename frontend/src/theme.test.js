import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

/**
 * The brand accent is a token, not a colour written into components -- JQ-72
 * removed the last hardcoded one. That makes a repaint a one-line change, and
 * it makes these rules checkable: every accent in the app resolves through the
 * declarations below, so if they hold, the whole app holds.
 *
 * Two failure modes this guards, both of which JQ-222 had to reason about by
 * hand:
 *
 *  - Repainting `--primary` and leaving `--ring`, `--sidebar-primary`,
 *    `--sidebar-ring` or `--accent-foreground` on the old colour. They are five
 *    copies of one decision and nothing but this test keeps them in step.
 *  - Picking an accent that reads well on the page background but fails on
 *    `--card`. Cards are where most accented text actually sits, and the gap
 *    between the two is small enough to miss by eye -- `#a855f7` clears AA on
 *    the background at 4.71 and fails on cards at 4.34.
 */

const AA = 4.5

// Read as text rather than importing it: this test reads the declarations in
// the file, it does not want Vite to process the Tailwind imports inside it.
const tokens = parseRootTokens(readFileSync(resolve(__dirname, 'tailwind.css'), 'utf8'))

function parseRootTokens(css) {
  const root = css.match(/:root\s*\{([\s\S]*?)\n\}/)
  if (!root) throw new Error('no :root block in tailwind.css')
  return Object.fromEntries(
    [...root[1].matchAll(/^\s*(--[\w-]+):\s*([^;]+);/gm)].map(([, name, value]) => [
      name,
      value.trim(),
    ]),
  )
}

function channel(c) {
  const s = c / 255
  return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
}

function luminance(hex) {
  const h = hex.replace('#', '')
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16))
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}

function contrast(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

function hue(hex) {
  const h = hex.replace('#', '')
  const [r, g, b] = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255)
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  if (max === min) return 0
  const d = max - min
  const deg =
    max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4
  return (deg * 60 + 360) % 360
}

/** The hue the designer's prototype uses for every magenta it carries: both
 *  `#be00ed` and `#9000b4` measure 288-289 degrees, differing only in lightness. */
const BRAND_HUE = 289
const HUE_TOLERANCE = 3

describe('brand accent', () => {
  it('is the designer magenta hue, not the teal it replaced', () => {
    expect(Math.abs(hue(tokens['--primary']) - BRAND_HUE)).toBeLessThanOrEqual(HUE_TOLERANCE)
  })

  it.each(['--ring', '--sidebar-primary', '--sidebar-ring', '--accent-foreground'])(
    '%s carries the same value as --primary',
    (token) => {
      expect(tokens[token]).toBe(tokens['--primary'])
    },
  )
})

describe('accent legibility', () => {
  it.each(['--background', '--card'])('--primary reads as text on %s', (surface) => {
    expect(contrast(tokens['--primary'], tokens[surface])).toBeGreaterThanOrEqual(AA)
  })

  it('--primary-foreground reads on a --primary fill', () => {
    expect(contrast(tokens['--primary-foreground'], tokens['--primary'])).toBeGreaterThanOrEqual(AA)
  })

  it.each(['--background', '--card'])('--destructive reads as text on %s', (surface) => {
    expect(contrast(tokens['--destructive'], tokens[surface])).toBeGreaterThanOrEqual(AA)
  })

  it('--destructive-foreground reads on a --destructive fill', () => {
    expect(contrast(tokens['--destructive-foreground'], tokens['--destructive'])).toBeGreaterThanOrEqual(AA)
  })
})

/**
 * A token nothing uses is not a guarantee. The rule above proves
 * `--destructive-foreground` is legible on a `--destructive` fill, but shadcn's
 * default ships `text-white` there instead -- 2.77:1. These two components are
 * the only places the app fills with `--destructive`.
 */
describe('destructive fills use the token, not white', () => {
  it.each(['components/ui/button-variants.js', 'components/ui/badge.jsx'])(
    '%s labels its --destructive fill with --destructive-foreground',
    (file) => {
      const source = readFileSync(resolve(__dirname, file), 'utf8')
      const fills = source.match(/'[^']*\bbg-destructive\b[^']*'/g)

      expect(fills).not.toBeNull()
      for (const fill of fills) {
        expect(fill).toContain('text-destructive-foreground')
        expect(fill).not.toContain('text-white')
      }
    },
  )
})

describe('links', () => {
  it.each(['--background', '--card'])('--link reads as text on %s', (surface) => {
    expect(contrast(tokens['--link'], tokens[surface])).toBeGreaterThanOrEqual(AA)
  })

  /**
   * Links are navigation and `--primary` is the option we want clicked, so the two
   * must not read as one colour where they sit together -- the footer, the
   * developer dashboard. Luminance alone is the check that catches the trap:
   * `#818cf8` against the new magenta measures 1.08, near-identical, while
   * against the old teal it measured 1.60 and looked fine.
   */
  it('is distinguishable from the brand accent', () => {
    expect(contrast(tokens['--link'], tokens['--primary'])).toBeGreaterThanOrEqual(1.3)
  })
})

/**
 * `--success` and `--warning` name an outcome, so they are almost always the text
 * of the thing they colour -- a "Winner" badge, an opted-in pill -- sitting on
 * their own tinted surface rather than on the page. That gives three surfaces to
 * clear, not one, and the surface is the tightest of them.
 */
describe('outcome colours', () => {
  it.each([
    ['--success', '--background'],
    ['--success', '--card'],
    ['--success', '--success-surface'],
    ['--warning', '--background'],
    ['--warning', '--card'],
    ['--warning', '--warning-surface'],
  ])('%s reads as text on %s', (token, surface) => {
    expect(contrast(tokens[token], tokens[surface])).toBeGreaterThanOrEqual(AA)
  })

  // The surfaces are panel backgrounds too, and a mood panel carries ordinary
  // body copy as well as its accent.
  it.each(['--success-surface', '--warning-surface'])('%s carries --foreground', (surface) => {
    expect(contrast(tokens['--foreground'], tokens[surface])).toBeGreaterThanOrEqual(AA)
  })

  /**
   * Green-for-good and amber-for-nearly are only legible as a pair if they are
   * actually different colours on screen. Luminance is the wrong measure here and
   * says so loudly -- the two sit at 1.04, near-identical -- because at the
   * lightness a dark theme needs, what separates them is entirely hue. That is the
   * opposite of the `--link` / `--primary` case above, where both are accent text
   * in a paragraph and hue alone does not carry.
   */
  it('are distinguishable from each other', () => {
    const apart = Math.abs(hue(tokens['--success']) - hue(tokens['--warning']))
    expect(Math.min(apart, 360 - apart)).toBeGreaterThanOrEqual(60)
  })
})
