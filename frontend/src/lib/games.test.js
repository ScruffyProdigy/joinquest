import { describe, it, expect } from 'vitest'
import {
  defaultModeForGame,
  isSoloMode,
  joinGroupOptionsForGame,
  joinGroupOptionsForMode,
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
