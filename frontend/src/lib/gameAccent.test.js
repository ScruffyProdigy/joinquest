import { describe, it, expect } from 'vitest'
import { accentBaseFor, accentColorFor, listAccentColors, parseHex } from './gameAccent'

describe('parseHex', () => {
  it('normalizes #rrggbb to lowercase', () => {
    expect(parseHex('#7C3AED')).toBe('#7c3aed')
  })

  it('expands #rgb shorthand to lowercase #rrggbb', () => {
    expect(parseHex('#ABC')).toBe('#aabbcc')
  })

  it('trims surrounding whitespace', () => {
    expect(parseHex('  #7c3aed  ')).toBe('#7c3aed')
  })

  it('returns null for unusable input', () => {
    for (const bad of ['', 'nope', '#12345', '#gggggg', 'rebeccapurple', 'rgb(1,2,3)', null, undefined]) {
      expect(parseHex(bad)).toBeNull()
    }
  })
})

describe('accentBaseFor', () => {
  it('returns the hashed palette entry as a plain hex', () => {
    expect(accentBaseFor('word-hunt')).toBe('#f59e0b')
  })

  it('returns a valid override instead of the hashed entry', () => {
    expect(accentBaseFor('word-hunt', '#7c3aed')).toBe('#7c3aed')
    expect(accentBaseFor('word-hunt', '#ABC')).toBe('#aabbcc')
  })

  it('falls back to the hashed entry when the override is unusable', () => {
    expect(accentBaseFor('word-hunt', '#12345')).toBe('#f59e0b')
    expect(accentBaseFor('word-hunt', '')).toBe('#f59e0b')
  })

  it('is the color accentColorFor builds its badge gradient from', () => {
    for (const override of [null, '#7c3aed']) {
      expect(accentColorFor('word-hunt', override).badge).toContain(
        accentBaseFor('word-hunt', override),
      )
    }
  })
})

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

  it('produces a badge gradient, card background, border, and header background for every result', () => {
    const result = accentColorFor('any-slug')
    expect(result.badge).toMatch(/^linear-gradient\(135deg, #[0-9a-f]{6}, #[0-9a-f]{6}\)$/)
    expect(result.cardBg).toMatch(/^linear-gradient\(160deg, color-mix\(/)
    expect(result.border).toMatch(/^#[0-9a-f]{6}40$/)
    expect(result.headerBg).toMatch(/^linear-gradient\(135deg, color-mix\(in oklab, .+ var\(--background\)\), color-mix\(in oklab, .+ var\(--background\)\)\)$/)
  })


  it('does not throw for an empty slug', () => {
    expect(() => accentColorFor('')).not.toThrow()
  })

  it('does not throw for an undefined slug', () => {
    expect(() => accentColorFor(undefined)).not.toThrow()
  })

  it('produces a heroScrim gradient tinted toward black for legibility', () => {
    const result = accentColorFor('any-slug')
    expect(result.heroScrim).toMatch(
      /^linear-gradient\(180deg, transparent 0%, color-mix\(in oklab, #[0-9a-f]{6} 55%, black\) 55%, color-mix\(in oklab, #[0-9a-f]{6} 80%, black\) 100%\)$/,
    )
  })
})

describe('accentColorFor with an explicit override', () => {
  it('uses the override instead of the slug-hashed palette entry', () => {
    const hashed = accentColorFor('some-game')
    const overridden = accentColorFor('some-game', '#7c3aed')

    expect(overridden).not.toEqual(hashed)
    expect(overridden.name).toBe('custom')
    expect(overridden.badge).toContain('#7c3aed')
  })

  it('derives a darker second gradient stop from the override', () => {
    // 0xff * 0.65 = 165.75, rounds to 166 = 0xa6
    expect(accentColorFor('some-game', '#ffffff').badge).toBe(
      'linear-gradient(135deg, #ffffff, #a6a6a6)',
    )
  })

  it('expands #rgb shorthand and ignores case', () => {
    expect(accentColorFor('some-game', '#ABC')).toEqual(accentColorFor('some-game', '#aabbcc'))
  })

  it('falls back to the hashed palette entry when the override is unparseable', () => {
    const hashed = accentColorFor('some-game')
    for (const bad of ['', 'nope', '#12345', '#gggggg', 'rebeccapurple', 'rgb(1,2,3)']) {
      expect(accentColorFor('some-game', bad)).toEqual(hashed)
    }
  })

  it('falls back to the hashed palette entry for null and undefined', () => {
    const hashed = accentColorFor('some-game')
    expect(accentColorFor('some-game', null)).toEqual(hashed)
    expect(accentColorFor('some-game', undefined)).toEqual(hashed)
  })

  it('keeps the same derived value shapes as a hashed accent', () => {
    const result = accentColorFor('some-game', '#7c3aed')

    expect(result.badge).toMatch(/^linear-gradient\(135deg, #[0-9a-f]{6}, #[0-9a-f]{6}\)$/)
    expect(result.border).toMatch(/^#[0-9a-f]{6}40$/)
    expect(result.cardBg).toMatch(/^linear-gradient\(160deg, color-mix\(/)
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

  it('gives every entry a badge, cardBg, border, and headerBg', () => {
    listAccentColors().forEach((entry) => {
      expect(entry.badge).toMatch(/^linear-gradient\(135deg, /)
      expect(entry.cardBg).toMatch(/^linear-gradient\(160deg, /)
      expect(entry.border).toMatch(/40$/)
      expect(entry.headerBg).toContain('color-mix')
      expect(entry.headerBg).toContain('var(--background)')
    })
  })

  it('blends headerBg toward --background so a single light foreground stays readable', () => {
    listAccentColors().forEach((entry) => {
      // Both gradient stops must be mixed toward --background (not the raw
      // vivid accent) so text-foreground has strong contrast at any position.
      expect(entry.headerBg).toMatch(/color-mix\(in oklab, #[0-9a-f]{6} 35%, var\(--background\)\)/)
      expect(entry.headerBg).toMatch(/color-mix\(in oklab, #[0-9a-f]{6} 45%, var\(--background\)\)/)
    })
  })

  it('gives every entry a heroScrim gradient', () => {
    listAccentColors().forEach((entry) => {
      expect(entry.heroScrim).toMatch(/^linear-gradient\(180deg, transparent 0%, color-mix\(/)
    })
  })
})
