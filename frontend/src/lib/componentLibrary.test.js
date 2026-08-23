import { describe, it, expect } from 'vitest'
import { PRIMITIVE_GROUPS, COMPOSITE_PATTERNS, statusStyle } from './componentLibrary'

const VALID_STATUSES = ['ported', 'legacy', 'not-started']

describe('statusStyle', () => {
  it('returns the ported style', () => {
    expect(statusStyle('ported')).toEqual({
      label: 'Ported',
      badgeClass: 'bg-primary/10 text-primary',
      dotClass: 'bg-primary',
    })
  })

  it('returns the legacy style', () => {
    expect(statusStyle('legacy')).toEqual({
      label: 'Legacy CSS',
      badgeClass: 'bg-amber-500/10 text-amber-500',
      dotClass: 'bg-amber-500',
    })
  })

  it('returns the not-started style for "not-started" and any unrecognized value', () => {
    const expected = {
      label: 'Not started',
      badgeClass: 'bg-foreground/5 text-muted-foreground',
      dotClass: 'bg-muted-foreground',
    }
    expect(statusStyle('not-started')).toEqual(expected)
    expect(statusStyle('unknown')).toEqual(expected)
  })
})

describe('PRIMITIVE_GROUPS', () => {
  it('gives every group a title, previewKind, and at least one item', () => {
    PRIMITIVE_GROUPS.forEach((group) => {
      expect(group.title).toBeTruthy()
      expect(group.previewKind).toBeTruthy()
      expect(group.items.length).toBeGreaterThan(0)
    })
  })

  it('gives every item a name, a valid status, and non-empty usage notes', () => {
    PRIMITIVE_GROUPS.forEach((group) => {
      group.items.forEach((item) => {
        expect(item.name).toBeTruthy()
        expect(VALID_STATUSES).toContain(item.status)
        expect(item.prototypeUse).toBeTruthy()
        expect(item.productionUse).toBeTruthy()
      })
    })
  })

  it('has exactly 15 primitives across 7 groups', () => {
    expect(PRIMITIVE_GROUPS).toHaveLength(7)
    const total = PRIMITIVE_GROUPS.reduce((sum, g) => sum + g.items.length, 0)
    expect(total).toBe(15)
  })

  it('marks Button, Avatar, Input, and Card as ported', () => {
    const items = PRIMITIVE_GROUPS.flatMap((g) => g.items)
    for (const name of ['Button', 'Avatar', 'Input', 'Card']) {
      expect(items.find((item) => item.name === name).status).toBe('ported')
    }
  })
})

describe('COMPOSITE_PATTERNS', () => {
  it('has exactly 5 patterns, each with a name, previewKind, valid status, and usage notes', () => {
    expect(COMPOSITE_PATTERNS).toHaveLength(5)
    COMPOSITE_PATTERNS.forEach((pattern) => {
      expect(pattern.name).toBeTruthy()
      expect(pattern.previewKind).toBeTruthy()
      expect(VALID_STATUSES).toContain(pattern.status)
      expect(pattern.prototypeUse).toBeTruthy()
      expect(pattern.productionUse).toBeTruthy()
    })
  })

  it('marks the avatar/spirit picker grid as ported', () => {
    const pattern = COMPOSITE_PATTERNS.find((p) => p.name === 'Avatar / spirit picker grid')
    expect(pattern.status).toBe('ported')
  })
})
