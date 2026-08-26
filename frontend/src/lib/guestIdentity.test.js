import { existsSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  GUEST_IDENTITY_CHOICES,
  SIGIL_FAMILIES,
  SIGIL_TINTS,
  generateGuestIdentities,
  generateGuestIdentity,
} from './guestIdentity'

describe('generateGuestIdentity', () => {
  it('pairs the name with a noun from its own sigil family', () => {
    for (const family of SIGIL_FAMILIES) {
      const identity = generateGuestIdentity(family)
      expect(identity.avatarKey).toMatch(new RegExp(`^sigil-${family.key}-`))
      expect(family.nouns.some((noun) => identity.name.includes(noun))).toBe(true)
    }
  })

  it('names the avatar after its own tint, so the colour matches the word', () => {
    for (const tint of SIGIL_TINTS) {
      const identity = generateGuestIdentity(SIGIL_FAMILIES[0], tint)
      expect(identity.name.startsWith(tint.word)).toBe(true)
      expect(identity.avatarKey).toBe(`sigil-canine-${tint.key}`)
      expect(identity.imageUrl).toBe(`/avatars/sigils/canine-${tint.key}.svg`)
    }
  })

  it('ends every name with four digits', () => {
    for (let i = 0; i < 50; i += 1) {
      const identity = generateGuestIdentity(SIGIL_FAMILIES[0])
      expect(identity.name).toMatch(/^[A-Z][a-z]+[A-Z][a-z]*\d{4}$/)
    }
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

  it('does not repeat a tint across the choices', () => {
    for (let i = 0; i < 20; i += 1) {
      const tints = generateGuestIdentities().map(
        (item) => SIGIL_TINTS.find((tint) => item.name.startsWith(tint.word)),
      )
      expect(new Set(tints).size).toBe(tints.length)
    }
  })
})

describe('sigil assets', () => {
  it('ships an SVG file for every family in every tint', () => {
    for (const family of SIGIL_FAMILIES) {
      for (const tint of SIGIL_TINTS) {
        expect(existsSync(`public/avatars/sigils/${family.key}-${tint.key}.svg`)).toBe(true)
      }
    }
  })

  it('paints each sigil in its own tint rather than a flat dark disc', () => {
    for (const tint of SIGIL_TINTS) {
      const svg = readFileSync(`public/avatars/sigils/canine-${tint.key}.svg`, 'utf8')
      expect(svg).toContain(tint.hex)
    }
  })

  it('uses families and tints the backend also accepts', () => {
    const catalog = readFileSync('../backend/internal/avatars/catalog.go', 'utf8')
    const listed = (name) => {
      const block = catalog.slice(catalog.indexOf(`var ${name} = `))
      return [...block.slice(0, block.indexOf('}')).matchAll(/"([a-z]+)"/g)].map((m) => m[1])
    }
    expect(listed('SigilFamilies').sort()).toEqual(SIGIL_FAMILIES.map((f) => f.key).sort())
    expect(listed('SigilTints').sort()).toEqual(SIGIL_TINTS.map((t) => t.key).sort())
  })
})
