/**
 * Writes the guest-tier sigil avatars: one SVG per animal family per tint.
 *
 * The tint is baked into the file rather than applied with CSS because game
 * clients render `avatarUrl` on their own backgrounds, where a transparent
 * silhouette could land invisible.
 *
 * Run after changing a shape or the palette:
 *   node scripts/generate-sigils.mjs
 */
import { mkdirSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { SIGIL_FAMILIES, SIGIL_SILHOUETTE, SIGIL_TINTS } from '../src/lib/guestIdentity.js'

/** Each shape draws a head in `body`, with cut-out details in `tint`. */
const SHAPES = {
  canine: (body, tint) => `
  <path d="M13 15 L23 25 L41 25 L51 15 L49 31 L44 41 L32 51 L20 41 L15 31 Z" fill="${body}"/>
  <circle cx="25.5" cy="32" r="2.4" fill="${tint}"/>
  <circle cx="38.5" cy="32" r="2.4" fill="${tint}"/>`,
  feline: (body, tint) => `
  <path d="M17 18 L24 27 L40 27 L47 18 L48 31 C48 42 41 49 32 49 C23 49 16 42 16 31 Z" fill="${body}"/>
  <circle cx="25" cy="33" r="2.4" fill="${tint}"/>
  <circle cx="39" cy="33" r="2.4" fill="${tint}"/>
  <path d="M32 39 L29 42 H35 Z" fill="${tint}"/>`,
  horned: (body, tint) => `
  <path d="M26 24 C18 23 12 19 9 11 C13 21 18 28 26 30 Z" fill="${body}"/>
  <path d="M38 24 C46 23 52 19 55 11 C51 21 46 28 38 30 Z" fill="${body}"/>
  <path d="M24 21 H40 L42 33 C42 42 38 49 32 49 C26 49 22 42 22 33 Z" fill="${body}"/>
  <circle cx="27" cy="31" r="2.4" fill="${tint}"/>
  <circle cx="37" cy="31" r="2.4" fill="${tint}"/>
  <ellipse cx="32" cy="43" rx="4.5" ry="3.2" fill="${tint}"/>`,
  raptor: (body, tint) => `
  <ellipse cx="26" cy="31" rx="13" ry="14" fill="${body}"/>
  <path d="M33 19 C42 19 48 21 52 25 C53 31 51 36 48 37 C47 31 43 28 35 29 Z" fill="${body}"/>
  <circle cx="24" cy="26" r="3" fill="${tint}"/>`,
  corvid: (body, tint) => `
  <path d="M18 28 C18 19 25 14 32 16 L57 27 L34 32 C35 40 30 47 24 46 C18 45 16 37 18 28 Z" fill="${body}"/>
  <circle cx="27" cy="26" r="2.8" fill="${tint}"/>`,
  ursine: (body, tint) => `
  <circle cx="19" cy="21" r="7.5" fill="${body}"/>
  <circle cx="45" cy="21" r="7.5" fill="${body}"/>
  <path d="M14 34 C14 24 22 18 32 18 C42 18 50 24 50 34 C50 44 42 50 32 50 C22 50 14 44 14 34 Z" fill="${body}"/>
  <circle cx="25" cy="31" r="2.4" fill="${tint}"/>
  <circle cx="39" cy="31" r="2.4" fill="${tint}"/>
  <ellipse cx="32" cy="41" rx="6" ry="4.5" fill="${tint}"/>`,
}

const outDir = resolve(dirname(fileURLToPath(import.meta.url)), '../public/avatars/sigils')
mkdirSync(outDir, { recursive: true })

let written = 0
for (const family of SIGIL_FAMILIES) {
  const shape = SHAPES[family.key]
  if (!shape) {
    throw new Error(`no shape defined for sigil family "${family.key}"`)
  }
  for (const tint of SIGIL_TINTS) {
    const label = `${tint.word} ${family.key} sigil`
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64" role="img" aria-label="${label}">
  <circle cx="32" cy="32" r="32" fill="${tint.hex}"/>${shape(SIGIL_SILHOUETTE, tint.hex)}
</svg>
`
    writeFileSync(resolve(outDir, `${family.key}-${tint.key}.svg`), svg)
    written += 1
  }
}

console.log(`wrote ${written} sigils to ${outDir}`)
