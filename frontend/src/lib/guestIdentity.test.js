import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'
import {
  GUEST_IDENTITY_CHOICES,
  SIGIL_FAMILIES,
  SIGIL_EXPRESSIONS,
  SIGIL_HUE_WORDS,
  generateGuestIdentities,
  generateGuestIdentity,
  generateTint,
  hueWord,
  perceivedHueOfRgb,
  relativeLuminance,
} from './guestIdentity'

const HEX = /^[0-9a-f]{6}$/

function hexToRgb(hex) {
  const bare = hex.replace('#', '')
  return [0, 2, 4].map((offset) => parseInt(bare.slice(offset, offset + 2), 16))
}

/** Runs `body` with every Math.random() draw pinned to `value`, so a draw repeats. */
function withRandom(value, body) {
  const spy = vi.spyOn(Math, 'random').mockReturnValue(value)
  try {
    return body()
  } finally {
    spy.mockRestore()
  }
}

describe('generateGuestIdentity', () => {
  it('pairs the name with a noun from its own sigil family', () => {
    for (const family of SIGIL_FAMILIES) {
      const identity = generateGuestIdentity(family)
      expect(identity.avatarKey).toMatch(
        new RegExp(`^sigil-${family.key}-[0-9a-f]{6}-(${SIGIL_EXPRESSIONS.join('|')})$`),
      )
      expect(family.nouns.some((noun) => identity.name.includes(noun))).toBe(true)
    }
  })

  it('names the avatar after its own slice of the wheel', () => {
    const slice = 360 / SIGIL_HUE_WORDS.length
    SIGIL_HUE_WORDS.forEach((word, index) => {
      const tint = generateTint(index * slice + slice / 2)
      const identity = generateGuestIdentity(SIGIL_FAMILIES[0], tint, 'wide')
      expect(identity.name.startsWith(word)).toBe(true)
      expect(identity.avatarKey).toBe(`sigil-canine-${tint.hex}-wide`)
      expect(identity.imageUrl).toBe(`/avatars/sigils/canine-${tint.hex}-wide.svg`)
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

  it('keeps every disc dark enough for the animal to be the lightest thing', () => {
    // Past this luminance no light mark can clear its contrast, and the renderer
    // flips the mark dark — which turns the animal into a hole in the disc instead
    // of a shape sitting on it. The contrast itself is the renderer's guarantee and
    // is covered in backend/internal/avatars/sigil_test.go.
    const FLIP = 0.1833
    const lums = Array.from({ length: 800 }, () =>
      relativeLuminance(hexToRgb(generateTint().hex)),
    )
    expect(Math.max(...lums)).toBeLessThan(FLIP)
    // It should still use the room it has rather than bunching at one depth.
    expect(Math.min(...lums)).toBeLessThan(0.07)
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
    // perceived hue. Two separate things keep that from coming back, and both are
    // exact — so this counts draws analytically rather than sampling them.
    const slice = 360 / SIGIL_HUE_WORDS.length

    // One: a guest's place on the wheel is the raw uniform draw scaled onto it, and
    // the wheel is cut into equal slices. Pinning the draw walks that mapping
    // directly, which is what a draw count was only ever estimating.
    SIGIL_HUE_WORDS.forEach((word, index) => {
      const middleOfSlice = (index + 0.5) / SIGIL_HUE_WORDS.length
      withRandom(middleOfSlice, () => expect(generateTint().word).toBe(word))
      expect(hueWord(index * slice)).toBe(word)
      expect(hueWord((index + 1) * slice - 1e-9)).toBe(word)
    })

    // Two: the slices have to be equal where the eye reads them, not just equal in
    // the parameter. This is the half the old scheme failed, and the half no number
    // of draws can see — cutting by HSL degrees still handed every word exactly one
    // slice in twenty-four. It only shows up in the colour that gets rendered.
    //
    // Saturation and lightness are pinned to both ends of their range and the middle,
    // since the hue is solved per draw against whichever the draw got.
    for (const draw of [0, 0.5, 0.999]) {
      const hues = withRandom(draw, () =>
        SIGIL_HUE_WORDS.map((_, index) =>
          perceivedHueOfRgb(hexToRgb(generateTint(index * slice).hex)),
        ),
      )
      const spans = hues.map((hue, i) => (hues[(i + 1) % hues.length] - hue + 360) % 360)
      // An even slice is 15 perceived degrees. Cutting by HSL degrees gave spans from
      // 3 (Ivy) to 31 (Lagoon); cutting by perceived hue holds every span between 9.6
      // and 18.8, the wobble being 8-bit rounding at the least saturated end. The
      // bounds clear that wobble and still exclude anything near the old imbalance.
      expect(Math.min(...spans)).toBeGreaterThan(8)
      expect(Math.max(...spans)).toBeLessThan(24)
    }
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

describe('expressions', () => {
  it('draws every face, and draws them independently of the colour', () => {
    // Independence is the point: tied to the colour, an expression would repeat on
    // exactly the pair of guests a shared colour already makes hard to tell apart.
    const seen = new Set()
    const perColour = new Set()
    const tint = generateTint(0)
    for (let i = 0; i < 400; i += 1) {
      seen.add(generateGuestIdentity(SIGIL_FAMILIES[0]).avatarKey.split('-').pop())
      perColour.add(generateGuestIdentity(SIGIL_FAMILIES[0], tint).avatarKey.split('-').pop())
    }
    expect([...seen].sort()).toEqual([...SIGIL_EXPRESSIONS].sort())
    expect(perColour.size).toBe(SIGIL_EXPRESSIONS.length)
  })

  it('multiplies the identities a table can tell apart', () => {
    const identities = new Set()
    for (let i = 0; i < 4000; i += 1) {
      const { avatarKey } = generateGuestIdentity(SIGIL_FAMILIES[0])
      identities.add(avatarKey.split('-').pop())
    }
    expect(identities.size).toBe(SIGIL_EXPRESSIONS.length)
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
