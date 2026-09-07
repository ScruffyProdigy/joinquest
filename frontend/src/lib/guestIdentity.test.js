import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  GUEST_IDENTITY_CHOICES,
  SIGIL_FAMILIES,
  SIGIL_HUE_WORDS,
  SIGIL_SILHOUETTE,
  generateGuestIdentities,
  generateGuestIdentity,
  generateTint,
  hueWord,
  relativeLuminance,
} from './guestIdentity'

const HEX = /^[0-9a-f]{6}$/

function hexToRgb(hex) {
  const bare = hex.replace('#', '')
  return [0, 2, 4].map((offset) => parseInt(bare.slice(offset, offset + 2), 16))
}

/** Hue in degrees, recovered from a rendered hex so tests read what guests see. */
function hueOf(hex) {
  const [r, g, b] = hexToRgb(hex).map((channel) => channel / 255)
  const max = Math.max(r, g, b)
  const span = max - Math.min(r, g, b)
  if (span === 0) {
    return 0
  }
  const raw =
    max === r ? (g - b) / span : max === g ? 2 + (b - r) / span : 4 + (r - g) / span
  return ((raw * 60) % 360 + 360) % 360
}

function contrastAgainstSilhouette(hex) {
  const silhouette = relativeLuminance(hexToRgb(SIGIL_SILHOUETTE))
  const disc = relativeLuminance(hexToRgb(hex))
  return (Math.max(silhouette, disc) + 0.05) / (Math.min(silhouette, disc) + 0.05)
}

describe('generateGuestIdentity', () => {
  it('pairs the name with a noun from its own sigil family', () => {
    for (const family of SIGIL_FAMILIES) {
      const identity = generateGuestIdentity(family)
      expect(identity.avatarKey).toMatch(new RegExp(`^sigil-${family.key}-[0-9a-f]{6}$`))
      expect(family.nouns.some((noun) => identity.name.includes(noun))).toBe(true)
    }
  })

  it('names the avatar after its own hue, so the word matches the colour', () => {
    for (const band of SIGIL_HUE_WORDS) {
      const tint = generateTint(band.until - 1)
      const identity = generateGuestIdentity(SIGIL_FAMILIES[0], tint)
      expect(identity.name.startsWith(band.word)).toBe(true)
      expect(identity.avatarKey).toBe(`sigil-canine-${tint.hex}`)
      expect(identity.imageUrl).toBe(`/avatars/sigils/canine-${tint.hex}.svg`)
    }
  })

  it('ends every name with four digits', () => {
    for (let i = 0; i < 50; i += 1) {
      const identity = generateGuestIdentity(SIGIL_FAMILIES[0])
      expect(identity.name).toMatch(/^[A-Z][a-z]+[A-Z][a-z]*\d{4}$/)
    }
  })
})

describe('generateTint', () => {
  it('always produces a 6-digit hex', () => {
    for (let i = 0; i < 200; i += 1) {
      expect(generateTint().hex).toMatch(HEX)
    }
  })

  it('stays legible under the near-white silhouette at every hue', () => {
    // HSL lightness is not perceptual, so an uncompensated yellow would wash the
    // silhouette out. Solving lightness for a target luminance keeps every hue in
    // the same contrast band.
    for (let hue = 0; hue < 360; hue += 3) {
      for (let i = 0; i < 5; i += 1) {
        const ratio = contrastAgainstSilhouette(generateTint(hue).hex)
        expect(ratio).toBeGreaterThanOrEqual(3)
        expect(ratio).toBeLessThan(5)
      }
    }
  })

  it('leans saturated without piling up at fully saturated', () => {
    const saturations = Array.from({ length: 400 }, () => {
      const [r, g, b] = hexToRgb(generateTint().hex)
      const max = Math.max(r, g, b)
      const min = Math.min(r, g, b)
      return max === 0 ? 0 : (max - min) / max
    })
    const mean = saturations.reduce((sum, value) => sum + value, 0) / saturations.length
    expect(mean).toBeGreaterThan(0.6)
    expect(saturations.filter((value) => value > 0.99).length / saturations.length).toBeLessThan(0.1)
  })

  it('covers the whole wheel', () => {
    const words = new Set(Array.from({ length: 600 }, () => generateTint().word))
    expect(words.size).toBe(new Set(SIGIL_HUE_WORDS.map((band) => band.word)).size)
  })
})

describe('hueWord', () => {
  it('wraps hues outside 0-360 back onto a band', () => {
    expect(hueWord(370)).toBe(hueWord(10))
    expect(hueWord(-10)).toBe(hueWord(350))
  })
})

describe('generateGuestIdentities', () => {
  it('offers one choice per sigil family, all distinct', () => {
    const identities = generateGuestIdentities()
    expect(identities).toHaveLength(GUEST_IDENTITY_CHOICES)
    expect(new Set(identities.map((item) => item.avatarKey)).size).toBe(GUEST_IDENTITY_CHOICES)
  })

  it('draws fresh names each time so guests do not collide', () => {
    const first = generateGuestIdentities().map((item) => item.name)
    const second = generateGuestIdentities().map((item) => item.name)
    expect(first).not.toEqual(second)
  })

  it('never offers more choices than there are families', () => {
    expect(generateGuestIdentities(99)).toHaveLength(SIGIL_FAMILIES.length)
  })

  it('spreads the choices around the colour wheel rather than clustering', () => {
    // One hue per equal sector, so the widest gap between neighbouring choices can
    // never span more than two sectors. Six uniform hues would blow past that.
    for (let i = 0; i < 50; i += 1) {
      const hues = generateGuestIdentities()
        .map((item) => hueOf(item.avatarKey.split('-').pop()))
        .sort((a, b) => a - b)
      const gaps = hues.map((hue, index) =>
        index === 0 ? hue + 360 - hues[hues.length - 1] : hue - hues[index - 1],
      )
      expect(Math.max(...gaps)).toBeLessThan(2 * (360 / GUEST_IDENTITY_CHOICES))
    }
  })
})

describe('sigil assets', () => {
  it('uses families the backend also draws', () => {
    const sigil = readFileSync('../backend/internal/avatars/sigil.go', 'utf8')
    const block = sigil.slice(sigil.indexOf('var SigilFamilies = '))
    const listed = [...block.slice(0, block.indexOf('}')).matchAll(/"([a-z]+)"/g)].map((m) => m[1])
    expect(listed.sort()).toEqual(SIGIL_FAMILIES.map((family) => family.key).sort())
  })
})
