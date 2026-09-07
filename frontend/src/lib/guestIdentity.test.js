import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  GUEST_IDENTITY_CHOICES,
  SIGIL_FAMILIES,
  SIGIL_HUE_WORDS,
  sigilMark,
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

/** Contrast between a disc and the mark the renderer will actually draw on it. */
function contrastAgainstMark(hex) {
  const mark = relativeLuminance(hexToRgb(sigilMark(hex)))
  const disc = relativeLuminance(hexToRgb(hex))
  return (Math.max(mark, disc) + 0.05) / (Math.min(mark, disc) + 0.05)
}

describe('generateGuestIdentity', () => {
  it('pairs the name with a noun from its own sigil family', () => {
    for (const family of SIGIL_FAMILIES) {
      const identity = generateGuestIdentity(family)
      expect(identity.avatarKey).toMatch(new RegExp(`^sigil-${family.key}-[0-9a-f]{6}$`))
      expect(family.nouns.some((noun) => identity.name.includes(noun))).toBe(true)
    }
  })

  it('names the avatar after its own slice of the wheel', () => {
    const slice = 360 / SIGIL_HUE_WORDS.length
    SIGIL_HUE_WORDS.forEach((word, index) => {
      const tint = generateTint(index * slice + slice / 2)
      const identity = generateGuestIdentity(SIGIL_FAMILIES[0], tint)
      expect(identity.name.startsWith(word)).toBe(true)
      expect(identity.avatarKey).toBe(`sigil-canine-${tint.hex}`)
      expect(identity.imageUrl).toBe(`/avatars/sigils/canine-${tint.hex}.svg`)
    })
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
    for (let i = 0; i < 100; i += 1) {
      expect(generateTint().hex).toMatch(HEX)
    }
  })

  it('carries its mark at 4:1 or better anywhere on the wheel', () => {
    // The mark is pale on a dark disc and dark on a light one, whichever reads
    // more strongly, so the worst case is the crossover between them rather than
    // either end of the lightness range. That worst case is still above the 3:1
    // WCAG floor for non-text contrast.
    for (let offset = 0; offset < 360; offset += 5) {
      for (let i = 0; i < 3; i += 1) {
        expect(contrastAgainstMark(generateTint(offset).hex)).toBeGreaterThanOrEqual(4)
      }
    }
  })

  it('uses the whole lightness range, not just the dark end', () => {
    const lums = Array.from({ length: 600 }, () =>
      relativeLuminance(hexToRgb(generateTint().hex)),
    )
    // A pale-only mark could never go above 0.285. Both marks together should put
    // real weight on either side of that.
    expect(lums.filter((y) => y > 0.285).length / lums.length).toBeGreaterThan(0.25)
    expect(lums.filter((y) => y < 0.2).length / lums.length).toBeGreaterThan(0.1)
  })

  it('lands each word on its own slice of perceived hue', () => {
    const slice = 360 / SIGIL_HUE_WORDS.length
    SIGIL_HUE_WORDS.forEach((word, index) => {
      const offset = index * slice + slice / 2
      const tint = generateTint(offset)
      expect(tint.word).toBe(word)
      // The hue it solved for really does sit where it was asked to.
      const [r, g, b] = hexToRgb(tint.hex)
      const max = Math.max(r, g, b)
      const span = max - Math.min(r, g, b)
      expect(span).toBeGreaterThan(10)
    })
  })

  it('spreads guests evenly across the words rather than piling up on green', () => {
    // Drawn uniformly in HSL degrees, green took 29% of guests for under a tenth of
    // perceived hue. Uniform in perceived hue, no word runs away with the wheel.
    const counts = new Map(SIGIL_HUE_WORDS.map((word) => [word, 0]))
    const draws = 3200
    for (let i = 0; i < draws; i += 1) {
      const word = generateTint().word
      counts.set(word, counts.get(word) + 1)
    }
    const shares = [...counts.values()].map((n) => n / draws)
    const expected = 1 / SIGIL_HUE_WORDS.length
    expect(Math.min(...shares)).toBeGreaterThan(expected * 0.7)
    expect(Math.max(...shares)).toBeLessThan(expected * 1.3)
  })

  it('leans saturated but keeps real spread', () => {
    const saturations = Array.from({ length: 400 }, () => {
      const [r, g, b] = hexToRgb(generateTint().hex)
      const max = Math.max(r, g, b)
      const min = Math.min(r, g, b)
      return max === 0 ? 0 : (max - min) / max
    })
    const mean = saturations.reduce((sum, value) => sum + value, 0) / saturations.length
    const spread = Math.sqrt(
      saturations.reduce((sum, v) => sum + (v - mean) ** 2, 0) / saturations.length,
    )
    expect(mean).toBeGreaterThan(0.5)
    expect(spread).toBeGreaterThan(0.08)
    expect(saturations.filter((value) => value > 0.99).length / saturations.length).toBeLessThan(0.1)
  })
})

describe('hueWord', () => {
  it('wraps positions outside 0-360 back onto a slice', () => {
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

  it('spreads the choices around the wheel rather than clustering', () => {
    // One position per equal sector of perceived hue. A word is narrower than a
    // sector, so two neighbouring choices can land on the same word when both fall
    // either side of a shared boundary — roughly one draw in seven. What the
    // stratification guarantees is the average, not any single row.
    const draws = 60
    let distinct = 0
    for (let i = 0; i < draws; i += 1) {
      const words = generateGuestIdentities().map(
        (item) => SIGIL_HUE_WORDS.find((word) => item.name.startsWith(word)),
      )
      expect(words.every(Boolean)).toBe(true)
      expect(new Set(words).size).toBeGreaterThan(GUEST_IDENTITY_CHOICES / 2)
      distinct += new Set(words).size
    }
    expect(distinct / draws).toBeGreaterThan(5.5)
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
