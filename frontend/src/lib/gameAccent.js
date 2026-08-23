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
    headerBg: `linear-gradient(135deg, color-mix(in oklab, ${entry.accent500} 35%, var(--background)), color-mix(in oklab, ${entry.accent700} 45%, var(--background)))`,
    heroScrim: `linear-gradient(180deg, transparent 0%, color-mix(in oklab, ${entry.accent700} 55%, black) 55%, color-mix(in oklab, ${entry.accent700} 80%, black) 100%)`,
  }
}

/** @param {string} slug @returns {number} */
function hashSlug(slug) {
  let hash = 0
  for (let i = 0; i < slug.length; i++) {
    hash = (hash * 31 + slug.charCodeAt(i)) | 0
  }
  return Math.abs(hash)
}

/**
 * Deterministic per-game accent, hashed from the game's slug.
 *
 * `overrideColor` is reserved for a future explicit per-game override (a
 * planned `games.accent_color` field, tracked as a follow-up ticket) and
 * has no effect yet — accepted now so this function's signature won't need
 * to change when that ticket lands.
 *
 * @param {string} slug
 * @param {string | null} [_overrideColor]
 * @returns {{name: string, badge: string, cardBg: string, border: string, headerBg: string, heroScrim: string}}
 */
export function accentColorFor(slug, _overrideColor = null) {
  const entry = ACCENT_PALETTE[hashSlug(slug) % ACCENT_PALETTE.length]
  return toAccent(entry)
}

/** @returns {Array<{name: string, badge: string, cardBg: string, border: string, headerBg: string, heroScrim: string}>} */
export function listAccentColors() {
  return ACCENT_PALETTE.map(toAccent)
}
