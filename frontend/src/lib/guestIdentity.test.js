import { existsSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  GUEST_IDENTITY_CHOICES,
  GUEST_NAME_ADJECTIVES,
  SIGIL_FAMILIES,
  generateGuestIdentities,
  generateGuestIdentity,
  isGeneratedDisplayName,
  needsIdentity,
} from './guestIdentity'

describe('generateGuestIdentity', () => {
  it('pairs the name with a noun from its own sigil family', () => {
    for (const family of SIGIL_FAMILIES) {
      const identity = generateGuestIdentity(family)
      expect(identity.avatarKey).toBe(family.key)
      expect(family.nouns.some((noun) => identity.name.includes(noun))).toBe(true)
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

  it('does not repeat an adjective across the choices', () => {
    for (let i = 0; i < 20; i += 1) {
      const adjectives = generateGuestIdentities().map(
        (item) => GUEST_NAME_ADJECTIVES.find((word) => item.name.startsWith(word)),
      )
      expect(new Set(adjectives).size).toBe(adjectives.length)
    }
  })
})

describe('isGeneratedDisplayName', () => {
  it.each([
    ['guest#123456', true],
    ['GUEST#000001', true],
    ['ryan (new)', true],
    ['', true],
    ['   ', true],
    [undefined, true],
    ['FrostFox4827', false],
    ['Ryan', false],
  ])('treats %s as generated: %s', (name, expected) => {
    expect(isGeneratedDisplayName(name)).toBe(expected)
  })
})

describe('needsIdentity', () => {
  const complete = { displayName: 'FrostFox4827', avatarKey: 'sigil-canine' }

  it('prompts a visitor with no session', () => {
    expect(needsIdentity(null)).toBe(true)
  })

  it('prompts a fresh guest who has neither a chosen name nor an avatar', () => {
    expect(needsIdentity({ displayName: 'guest#421900', avatarKey: '' })).toBe(true)
  })

  it('prompts when only the avatar is missing', () => {
    expect(needsIdentity({ displayName: 'Ryan', avatarKey: '' })).toBe(true)
  })

  it('prompts when only the name is still generated', () => {
    expect(needsIdentity({ displayName: 'guest#421900', avatarKey: 'sigil-canine' })).toBe(true)
  })

  it('leaves a guest who already picked both alone', () => {
    expect(needsIdentity({ ...complete, isGuest: true })).toBe(false)
  })

  it('leaves a signed-in player with a spirit animal alone', () => {
    expect(needsIdentity({ displayName: 'Ryan', avatarUrl: 'https://cdn/spirit.png' })).toBe(false)
  })
})

describe('sigil assets', () => {
  it('ships an SVG file for every family', () => {
    for (const family of SIGIL_FAMILIES) {
      expect(existsSync(`public${family.imageUrl}`)).toBe(true)
    }
  })

  it('uses the same keys as the backend catalog', () => {
    const catalog = readFileSync('../backend/internal/avatars/catalog.go', 'utf8')
    const backendKeys = [...catalog.matchAll(/Key: "(sigil-[a-z]+)"/g)].map((match) => match[1])
    expect(backendKeys.sort()).toEqual(SIGIL_FAMILIES.map((family) => family.key).sort())
  })
})
