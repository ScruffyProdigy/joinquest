import { describe, it, expect } from 'vitest'
import {
  gameCatalogHeroUrl,
  gameDetailDescription,
  gameGenreModeLabel,
  gameLiveActivityLabel,
  gameHeroUrl,
  gameIconUrl,
  gamePagePath,
  gamePageShareUrl,
  gamePlayerCountLabel,
  gameTagChips,
} from './gameCard'

describe('gameCard', () => {
  it('gameIconUrl returns the icon path', () => {
    expect(gameIconUrl({ iconUrl: '/games/rpslr-icon.png' })).toBe('/games/rpslr-icon.png?v=1')
  })

  it('gameIconUrl throws when missing', () => {
    expect(() => gameIconUrl({})).toThrow('game iconUrl is required')
  })

  it('gameHeroUrl returns the hero path', () => {
    expect(gameHeroUrl({ heroUrl: '/games/rpslr-hero.jpg' })).toBe('/games/rpslr-hero.jpg?v=1')
  })

  it('gameHeroUrl throws when missing', () => {
    expect(() => gameHeroUrl({})).toThrow('game heroUrl is required')
  })

  it('gameCatalogHeroUrl prefers catalogHeroUrl', () => {
    expect(
      gameCatalogHeroUrl({
        catalogHeroUrl: '/games/catalog.png',
        heroUrl: '/games/rpslr-hero.jpg',
      }),
    ).toBe('/games/catalog.png?v=1')
  })

  it('gameCatalogHeroUrl falls back to heroUrl', () => {
    expect(gameCatalogHeroUrl({ heroUrl: '/games/rpslr-hero.jpg' })).toBe('/games/rpslr-hero.jpg?v=1')
  })

  it('gamePagePath returns a shareable path', () => {
    expect(gamePagePath({ slug: 'word-hunt' })).toBe('/games/word-hunt')
  })

  it('gamePagePath returns null without a slug', () => {
    expect(gamePagePath({})).toBeNull()
  })

  it('gamePageShareUrl returns an absolute URL', () => {
    expect(gamePageShareUrl({ slug: 'word-hunt' })).toMatch(/\/games\/word-hunt$/)
  })

  it('gameDetailDescription prefers longDescription', () => {
    expect(
      gameDetailDescription({
        longDescription: 'Long copy.',
        shortDescription: 'Short copy.',
      }),
    ).toBe('Long copy.')
  })

  it('gameDetailDescription falls back to shortDescription', () => {
    expect(gameDetailDescription({ shortDescription: 'Short copy.' })).toBe('Short copy.')
  })

  it('gameTagChips caps at three tags', () => {
    expect(gameTagChips(['a', 'b', 'c', 'd'])).toEqual(['a', 'b', 'c'])
  })

  it('gameGenreModeLabel joins the first two tags', () => {
    expect(gameGenreModeLabel(['Trivia', 'Party', 'extra'])).toBe('Trivia · Party')
  })

  it('gameGenreModeLabel uses a single tag alone', () => {
    expect(gameGenreModeLabel(['Trivia'])).toBe('Trivia')
  })

  it('gameGenreModeLabel returns null with no tags', () => {
    expect(gameGenreModeLabel([])).toBeNull()
    expect(gameGenreModeLabel(undefined)).toBeNull()
  })

  it('gamePlayerCountLabel ranges across active modes', () => {
    expect(
      gamePlayerCountLabel([
        { status: 'active', minPlayers: 2, maxPlayers: 8 },
        { status: 'inactive', minPlayers: 1, maxPlayers: 1 },
      ]),
    ).toBe('2–8 players')
  })

  it('gamePlayerCountLabel collapses to a single count when min equals max', () => {
    expect(gamePlayerCountLabel([{ status: 'active', minPlayers: 2, maxPlayers: 2 }])).toBe('2 players')
  })

  it('gamePlayerCountLabel uses singular phrasing for one player', () => {
    expect(gamePlayerCountLabel([{ status: 'active', minPlayers: 1, maxPlayers: 1 }])).toBe('1 player')
  })

  it('gamePlayerCountLabel returns null with no active modes', () => {
    expect(gamePlayerCountLabel([{ status: 'inactive', minPlayers: 2, maxPlayers: 4 }])).toBeNull()
    expect(gamePlayerCountLabel([])).toBeNull()
    expect(gamePlayerCountLabel(undefined)).toBeNull()
  })

  it('gameLiveActivityLabel prefers players in a session', () => {
    expect(gameLiveActivityLabel({ playing: 4, queued: 2 })).toBe('4 playing')
    expect(gameLiveActivityLabel({ playing: 1, queued: 0 })).toBe('1 playing')
  })

  it('gameLiveActivityLabel falls back to the queue when nobody is playing', () => {
    expect(gameLiveActivityLabel({ playing: 0, queued: 2 })).toBe('2 waiting')
  })

  it('gameLiveActivityLabel returns null for a quiet game', () => {
    expect(gameLiveActivityLabel({ playing: 0, queued: 0 })).toBeNull()
    expect(gameLiveActivityLabel(undefined)).toBeNull()
  })
})
