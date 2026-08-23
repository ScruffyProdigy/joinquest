const ACCENT_PALETTE = [
  { name: 'amber', accent500: '#f59e0b', accent700: '#b45309' },
  { name: 'orange', accent500: '#f97316', accent700: '#c2410c' },
  { name: 'lime', accent500: '#84cc16', accent700: '#4d7c0f' },
  { name: 'emerald', accent500: '#10b981', accent700: '#047857' },
  { name: 'cyan', accent500: '#06b6d4', accent700: '#0e7490' },
  { name: 'blue', accent500: '#3b82f6', accent700: '#1d4ed8' },
  { name: 'indigo', accent500: '#6366f1', accent700: '#4338ca' },
  { name: 'violet', accent500: '#8b5cf6', accent700: '#6d28d9' },
  { name: 'fuchsia', accent500: '#d946ef', accent700: '#a21caf' },
  { name: 'rose', accent500: '#f43f5e', accent700: '#be123c' },
]

/** @param {{name: string, accent500: string, accent700: string}} entry */
function toAccent(entry) {
  return {
    name: entry.name,
    badge: `linear-gradient(135deg, ${entry.accent500}, ${entry.accent700})`,
    cardBg: `linear-gradient(160deg, color-mix(in oklab, ${entry.accent700} 12%, var(--card)), color-mix(in oklab, ${entry.accent700} 6%, var(--card)))`,
    border: `${entry.accent500}40`,
    // Blended well toward --background so both gradient stops stay dark
    // regardless of hue, letting a single fixed light foreground color
    // (text-foreground) always have strong contrast, at any text position.
    // That guarantee holds for the palette above; a developer-supplied
    // override (e.g. a near-white hex) can still come out lighter.
    headerBg: `linear-gradient(135deg, color-mix(in oklab, ${entry.accent500} 35%, var(--background)), color-mix(in oklab, ${entry.accent700} 45%, var(--background)))`,
    heroScrim: `linear-gradient(180deg, transparent 0%, color-mix(in oklab, ${entry.accent700} 55%, black) 55%, color-mix(in oklab, ${entry.accent700} 80%, black) 100%)`,
  }
}

/** Accept `#rgb` or `#rrggbb` in any case; return a normalized `#rrggbb`, or null if unusable. */
export function parseHex(value) {
  const raw = String(value ?? '').trim().toLowerCase()
  const short = /^#([0-9a-f])([0-9a-f])([0-9a-f])$/.exec(raw)
  if (short) {
    return `#${short[1]}${short[1]}${short[2]}${short[2]}${short[3]}${short[3]}`
  }
  return /^#[0-9a-f]{6}$/.test(raw) ? raw : null
}

/** Darken toward black by roughly the gap between the palette's 500 and 700 stops. */
function darken(hex) {
  const channels = [1, 3, 5].map((i) => Math.round(parseInt(hex.slice(i, i + 2), 16) * 0.65))
  return `#${channels.map((c) => c.toString(16).padStart(2, '0')).join('')}`
}

/** @param {string} slug @returns {number} */
function hashSlug(slug) {
  const s = String(slug ?? '')
  let hash = 0
  for (let i = 0; i < s.length; i++) {
    hash = (hash * 31 + s.charCodeAt(i)) | 0
  }
  return Math.abs(hash)
}

/**
 * The palette entry a game draws its accent from: a valid `overrideColor` when
 * present, otherwise one deterministically hashed from the slug.
 *
 * The override is re-validated here rather than trusted, so bad stored data
 * degrades to the hashed accent instead of blanking out a card.
 *
 * @param {string} slug
 * @param {string | null} overrideColor
 * @returns {{name: string, accent500: string, accent700: string}}
 */
function entryFor(slug, overrideColor) {
  const override = parseHex(overrideColor)
  if (override) {
    return { name: 'custom', accent500: override, accent700: darken(override) }
  }
  return ACCENT_PALETTE[hashSlug(slug) % ACCENT_PALETTE.length]
}

/**
 * Per-game accent as ready-to-use CSS values.
 *
 * @param {string} slug
 * @param {string | null} [overrideColor]
 * @returns {{name: string, badge: string, cardBg: string, border: string, headerBg: string, heroScrim: string}}
 */
export function accentColorFor(slug, overrideColor = null) {
  return toAccent(entryFor(slug, overrideColor))
}

/**
 * The same accent as a plain hex, for UI that needs a raw color rather than a
 * CSS value — a `<input type="color">` swatch, for instance.
 *
 * @param {string} slug
 * @param {string | null} [overrideColor]
 * @returns {string}
 */
export function accentBaseFor(slug, overrideColor = null) {
  return entryFor(slug, overrideColor).accent500
}

/** @returns {Array<{name: string, badge: string, cardBg: string, border: string, headerBg: string, heroScrim: string}>} */
export function listAccentColors() {
  return ACCENT_PALETTE.map(toAccent)
}
