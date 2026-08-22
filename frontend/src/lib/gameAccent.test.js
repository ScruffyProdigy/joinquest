import { describe, it, expect } from 'vitest'
import { accentColorFor, listAccentColors } from './gameAccent'

describe('accentColorFor', () => {
  it('is deterministic for the same slug', () => {
    const first = accentColorFor('word-hunt')
    const second = accentColorFor('word-hunt')
    expect(second).toEqual(first)
  })

  it('returns one of the 10 known palette families', () => {
    const known = listAccentColors().map((c) => c.name)
    const result = accentColorFor('trivia-blitz')
    expect(known).toContain(result.name)
  })

  it('distributes across multiple families for a varied set of slugs', () => {
    const slugs = [
      'trivia-blitz', 'word-hunt', 'card-clash', 'social-deduction',
      'fighting-fury', 'strategy-siege', 'party-pack', 'quick-draw',
      'co-op-quest', 'solo-run', 'team-trivia', 'duel-arena',
    ]
    const distinctNames = new Set(slugs.map((slug) => accentColorFor(slug).name))
    expect(distinctNames.size).toBeGreaterThanOrEqual(5)
  })

  it('produces a badge gradient, card background, and border for every result', () => {
    const result = accentColorFor('any-slug')
    expect(result.badge).toMatch(/^linear-gradient\(135deg, #[0-9a-f]{6}, #[0-9a-f]{6}\)$/)
    expect(result.cardBg).toMatch(/^linear-gradient\(160deg, color-mix\(/)
    expect(result.border).toMatch(/^#[0-9a-f]{6}40$/)
  })

  it('ignores the overrideColor param (reserved for a future ticket)', () => {
    const withoutOverride = accentColorFor('some-game')
    const withOverride = accentColorFor('some-game', '#000000')
    expect(withOverride).toEqual(withoutOverride)
  })

  it('does not throw for an empty slug', () => {
    expect(() => accentColorFor('')).not.toThrow()
  })
})

describe('listAccentColors', () => {
  it('returns exactly the 10 fixed palette families', () => {
    const names = listAccentColors().map((c) => c.name)
    expect(names).toEqual([
      'amber', 'orange', 'lime', 'emerald', 'cyan',
      'blue', 'indigo', 'violet', 'fuchsia', 'rose',
    ])
  })

  it('gives every entry a badge, cardBg, and border', () => {
    listAccentColors().forEach((entry) => {
      expect(entry.badge).toMatch(/^linear-gradient\(135deg, /)
      expect(entry.cardBg).toMatch(/^linear-gradient\(160deg, /)
      expect(entry.border).toMatch(/40$/)
    })
  })
})
