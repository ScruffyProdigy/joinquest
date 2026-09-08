import { describe, it, expect } from 'vitest'
import {
  defaultModeForGame,
  filterGamesBySearch,
  isSoloMode,
  joinGroupOptionsForGame,
  joinGroupOptionsForMode,
  modePlayerRangeLabel,
} from './games'

describe('joinGroupOptionsForMode', () => {
  it('returns single-bucket (fifo) when all seats have empty queue paths', () => {
    const mode = {
      seats: [{ queuePath: null }, { queuePath: '' }],
    }
    expect(joinGroupOptionsForMode(mode)).toEqual({ kind: 'fifo', paths: [] })
  })

  it('returns sorted composition paths', () => {
    const mode = {
      seats: [
        { queuePath: 'Tank' },
        { queuePath: 'DPS' },
        { queuePath: 'DPS' },
        { queuePath: 'Support' },
      ],
    }
    expect(joinGroupOptionsForMode(mode)).toEqual({
      kind: 'composition',
      paths: [
        { queuePath: 'DPS', displayName: 'DPS' },
        { queuePath: 'Support', displayName: 'Support' },
        { queuePath: 'Tank', displayName: 'Tank' },
      ],
    })
  })

  it('treats empty queuePaths as fifo (single-bucket modes)', () => {
    const mode = {
      queuePaths: [{ queuePath: '', displayName: '', playersToStart: 2 }],
      seats: [{ queuePath: null }, { queuePath: null }],
    }
    expect(joinGroupOptionsForMode(mode)).toEqual({ kind: 'fifo', paths: [] })
  })

  it('prefers queuePaths metadata when present', () => {
    const mode = {
      queuePaths: [
        { queuePath: 'ClueGiver', displayName: 'Clue Giver', playersToStart: 2 },
        { queuePath: 'Guesser', displayName: 'Guesser', playersToStart: 4 },
      ],
      seats: [{ queuePath: 'ClueGiver' }, { queuePath: 'Guesser' }],
    }
    expect(joinGroupOptionsForMode(mode)).toEqual({
      kind: 'composition',
      paths: [
        { queuePath: 'ClueGiver', displayName: 'Clue Giver' },
        { queuePath: 'Guesser', displayName: 'Guesser' },
      ],
    })
  })
})

describe('joinGroupOptionsForGame', () => {
  it('uses the default active mode', () => {
    const game = {
      modes: [
        {
          queues: [{ status: 'inactive' }],
          seats: [{ queuePath: 'DPS' }],
        },
        {
          queues: [{ status: 'active' }],
          seats: [{ queuePath: 'Tank' }, { queuePath: 'DPS' }],
        },
      ],
    }
    expect(defaultModeForGame(game).queues[0].status).toBe('active')
    expect(joinGroupOptionsForGame(game)).toEqual({
      kind: 'composition',
      paths: [
        { queuePath: 'DPS', displayName: 'DPS' },
        { queuePath: 'Tank', displayName: 'Tank' },
      ],
    })
  })
})

describe('isSoloMode', () => {
  it('is true when a mode has exactly one seat', () => {
    expect(isSoloMode({ minPlayers: 1, maxPlayers: 1 })).toBe(true)
  })

  it('is false for multiplayer modes', () => {
    expect(isSoloMode({ minPlayers: 2, maxPlayers: 2 })).toBe(false)
    expect(isSoloMode({ minPlayers: 2, maxPlayers: 5 })).toBe(false)
  })

  it('is false when player bounds are missing', () => {
    expect(isSoloMode({})).toBe(false)
    expect(isSoloMode(undefined)).toBe(false)
  })
})

describe('modePlayerRangeLabel', () => {
  it('formats a range when min and max differ', () => {
    expect(modePlayerRangeLabel({ minPlayers: 2, maxPlayers: 4 })).toBe('2-4 players')
  })

  it('formats a single count when min and max match and are not 1', () => {
    expect(modePlayerRangeLabel({ minPlayers: 8, maxPlayers: 8 })).toBe('8 players')
  })

  it('uses singular "player" for an exact count of 1', () => {
    expect(modePlayerRangeLabel({ minPlayers: 1, maxPlayers: 1 })).toBe('1 player')
  })

  it('returns null when minPlayers is missing', () => {
    expect(modePlayerRangeLabel({ maxPlayers: 4 })).toBeNull()
  })

  it('returns null when maxPlayers is missing', () => {
    expect(modePlayerRangeLabel({ minPlayers: 2 })).toBeNull()
  })

  it('returns null for undefined mode', () => {
    expect(modePlayerRangeLabel(undefined)).toBeNull()
  })
})

describe('filterGamesBySearch', () => {
  const games = [
    { id: '1', name: 'Spyfall', genre: 'deduction', modes: [{ socialMode: 'hidden-roles' }] },
    { id: '2', name: 'Word Ladder', genre: 'words-trivia', difficulty: 'casual' },
    { id: '3', name: 'Rock Paper Scissors Lizard Robot' },
  ]

  it('returns every game for an empty or whitespace-only query', () => {
    expect(filterGamesBySearch(games, '')).toEqual(games)
    expect(filterGamesBySearch(games, '   ')).toEqual(games)
    expect(filterGamesBySearch(games, null)).toEqual(games)
  })

  it('matches game titles case-insensitively', () => {
    expect(filterGamesBySearch(games, 'spyFALL').map((g) => g.id)).toEqual(['1'])
  })

  it('matches partial titles', () => {
    expect(filterGamesBySearch(games, 'lad').map((g) => g.id)).toEqual(['2'])
  })

  it('matches axis labels as well as titles', () => {
    expect(filterGamesBySearch(games, 'deduction').map((g) => g.id)).toEqual(['1'])
    // "Hidden roles" is a mode's social shape, not the game's genre.
    expect(filterGamesBySearch(games, 'hidden').map((g) => g.id)).toEqual(['1'])
    expect(filterGamesBySearch(games, 'casual').map((g) => g.id)).toEqual(['2'])
  })

  it('requires every term to match', () => {
    expect(filterGamesBySearch(games, 'word trivia').map((g) => g.id)).toEqual(['2'])
    expect(filterGamesBySearch(games, 'word deduction')).toEqual([])
  })

  it('returns an empty list when nothing matches', () => {
    expect(filterGamesBySearch(games, 'zzzzz')).toEqual([])
  })

  it('tolerates games without axes and a missing list', () => {
    expect(filterGamesBySearch(games, 'lizard').map((g) => g.id)).toEqual(['3'])
    expect(filterGamesBySearch(undefined, 'lizard')).toEqual([])
  })
})
