const ACCENT_PALETTE = [
  { name: 'amber', accent500: '#f59e0b', accent700: '#b45309', foreground: '#071210' },
  { name: 'orange', accent500: '#f97316', accent700: '#c2410c', foreground: '#071210' },
  { name: 'lime', accent500: '#84cc16', accent700: '#4d7c0f', foreground: '#071210' },
  { name: 'emerald', accent500: '#10b981', accent700: '#047857', foreground: '#f0fdf4' },
  { name: 'cyan', accent500: '#06b6d4', accent700: '#0e7490', foreground: '#f0fdfa' },
  { name: 'blue', accent500: '#3b82f6', accent700: '#1d4ed8', foreground: '#eff6ff' },
  { name: 'indigo', accent500: '#6366f1', accent700: '#4338ca', foreground: '#eef2ff' },
  { name: 'violet', accent500: '#8b5cf6', accent700: '#6d28d9', foreground: '#f5f3ff' },
  { name: 'fuchsia', accent500: '#d946ef', accent700: '#a21caf', foreground: '#fdf4ff' },
  { name: 'rose', accent500: '#f43f5e', accent700: '#be123c', foreground: '#fff1f2' },
]

/** @param {{name: string, accent500: string, accent700: string, foreground: string}} entry */
function toAccent(entry) {
  return {
    name: entry.name,
    badge: `linear-gradient(135deg, ${entry.accent500}, ${entry.accent700})`,
    cardBg: `linear-gradient(160deg, color-mix(in oklab, ${entry.accent700} 12%, var(--card)), color-mix(in oklab, ${entry.accent700} 6%, var(--card)))`,
    border: `${entry.accent500}40`,
    foreground: entry.foreground,
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
 * @returns {{name: string, badge: string, cardBg: string, border: string, foreground: string}}
 */
export function accentColorFor(slug, _overrideColor = null) {
  const entry = ACCENT_PALETTE[hashSlug(slug) % ACCENT_PALETTE.length]
  return toAccent(entry)
}

/** @returns {Array<{name: string, badge: string, cardBg: string, border: string, foreground: string}>} */
export function listAccentColors() {
  return ACCENT_PALETTE.map(toAccent)
}
