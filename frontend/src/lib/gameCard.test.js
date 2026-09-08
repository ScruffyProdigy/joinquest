import { describe, it, expect } from 'vitest'
import {
  gameCatalogHeroUrl,
  gameDetailDescription,
  gameAxisChips,
  gameDifficultyLabel,
  gameGenreLabel,
  gameLiveActivityLabel,
  gameHeroUrl,
  gameIconUrl,
  gamePagePath,
  gamePageShareUrl,
  gamePlayerRangeLabel,
  modeSocialModeLabel,
  gameCardMeta,
  gameDurationLabel,
  gameTitleArtStyle,
  gameTitleArtUrl,
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

  it('gameCatalogHeroUrl falls back to the placeholder rather than throwing', () => {
    expect(gameCatalogHeroUrl({})).toBe('/games/default-hero.svg?v=1')
    expect(gameCatalogHeroUrl({ heroUrl: '   ' })).toBe('/games/default-hero.svg?v=1')
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

  it('gameGenreLabel renders the genre axis label', () => {
    expect(gameGenreLabel({ genre: 'words-trivia' })).toBe('Words & Trivia')
  })

  it('gameGenreLabel returns null for an unset or retired genre', () => {
    expect(gameGenreLabel({})).toBeNull()
    expect(gameGenreLabel(undefined)).toBeNull()
    // `words` was a tag id, never a genre id; it must not render as a slug.
    expect(gameGenreLabel({ genre: 'words' })).toBeNull()
  })

  it('gameDifficultyLabel renders the difficulty axis label', () => {
    expect(gameDifficultyLabel({ difficulty: 'casual' })).toBe('Casual')
    expect(gameDifficultyLabel({})).toBeNull()
  })

  it('modeSocialModeLabel renders the mode-level social shape', () => {
    expect(modeSocialModeLabel({ socialMode: '1v1' })).toBe('1v1')
    expect(modeSocialModeLabel({ socialMode: 'free-for-all' })).toBe('Free-for-all')
    expect(modeSocialModeLabel({})).toBeNull()
  })

  it('gameAxisChips orders genre, social mode, then difficulty', () => {
    expect(
      gameAxisChips({ genre: 'words-trivia', difficulty: 'casual' }, { socialMode: '1v1' }),
    ).toEqual(['Words & Trivia', '1v1', 'Casual'])
  })

  it('gameAxisChips drops axes the game has not declared', () => {
    expect(gameAxisChips({ genre: 'puzzle' })).toEqual(['Puzzle'])
    expect(gameAxisChips({})).toEqual([])
  })

  it('gamePlayerRangeLabel ranges across active modes', () => {
    expect(
      gamePlayerRangeLabel([
        { status: 'active', minPlayers: 2, maxPlayers: 8 },
        { status: 'inactive', minPlayers: 1, maxPlayers: 1 },
      ]),
    ).toBe('2–8')
  })

  it('gamePlayerRangeLabel collapses to a single count when min equals max', () => {
    expect(gamePlayerRangeLabel([{ status: 'active', minPlayers: 2, maxPlayers: 2 }])).toBe('2')
  })

  it('gamePlayerRangeLabel returns null with no active modes', () => {
    expect(gamePlayerRangeLabel([{ status: 'inactive', minPlayers: 2, maxPlayers: 4 }])).toBeNull()
    expect(gamePlayerRangeLabel([])).toBeNull()
    expect(gamePlayerRangeLabel(undefined)).toBeNull()
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

  it('gameDurationLabel formats a session length', () => {
    expect(gameDurationLabel({ durationMinutes: 12 })).toBe('12 min')
  })

  it('gameDurationLabel returns null until a game carries a duration', () => {
    expect(gameDurationLabel({})).toBeNull()
    expect(gameDurationLabel({ durationMinutes: 0 })).toBeNull()
    expect(gameDurationLabel(undefined)).toBeNull()
  })

  it('gameCardMeta orders genre, then players, then duration', () => {
    const meta = gameCardMeta({
      genre: 'words-trivia',
      modes: [{ status: 'active', minPlayers: 2, maxPlayers: 8 }],
      durationMinutes: 12,
    })

    expect(meta.map((pill) => pill.key)).toEqual(['genre', 'players', 'duration'])
    expect(meta.map((pill) => pill.label)).toEqual(['Words & Trivia', '2–8', '12 min'])
  })

  it('gameCardMeta drops absent facts instead of leaving empty slots', () => {
    const meta = gameCardMeta({
      modes: [{ status: 'active', minPlayers: 2, maxPlayers: 8 }],
    })

    expect(meta.map((pill) => pill.key)).toEqual(['players'])
  })

  it('gameCardMeta is empty when a game carries none of the three', () => {
    expect(gameCardMeta({})).toEqual([])
  })

  it('gameTitleArtUrl versions the wordmark like the rest of the art', () => {
    expect(gameTitleArtUrl({ url: '/games/word-hunt-title.webp' })).toBe(
      '/games/word-hunt-title.webp?v=1',
    )
    expect(gameTitleArtUrl(null)).toBeNull()
  })

  it('gameTitleArtStyle pins a bottom-left mark to the card inset', () => {
    const style = gameTitleArtStyle({ url: '/t.webp', anchor: 'bottom-left', widthPct: 70 })

    expect(style).toMatchObject({
      position: 'absolute',
      bottom: '6.8%',
      left: '5.5%',
      width: '70%',
      maxHeight: '26%',
      objectFit: 'contain',
    })
    expect(style.top).toBeUndefined()
    expect(style.transform).toBeUndefined()
  })

  it('gameTitleArtStyle centres a middle-center mark on both axes', () => {
    const style = gameTitleArtStyle({ url: '/t.webp', anchor: 'middle-center', widthPct: 34 })

    expect(style.top).toBe('50%')
    expect(style.left).toBe('50%')
    expect(style.transform).toBe('translateX(-50%) translateY(-50%)')
  })

  it('gameTitleArtStyle clamps a width past the card edge', () => {
    expect(gameTitleArtStyle({ url: '/t.webp', anchor: 'top-left', widthPct: 140 }).width).toBe('100%')
  })

  it('gameTitleArtStyle returns null without a usable placement', () => {
    expect(gameTitleArtStyle(null)).toBeNull()
    expect(gameTitleArtStyle({ url: '/t.webp', anchor: 'nowhere', widthPct: 70 })).toBeNull()
    expect(gameTitleArtStyle({ url: '', anchor: 'top-left', widthPct: 70 })).toBeNull()
    expect(gameTitleArtStyle({ url: '/t.webp', anchor: 'top-left', widthPct: 0 })).toBeNull()
  })
})
