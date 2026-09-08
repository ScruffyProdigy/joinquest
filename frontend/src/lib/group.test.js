import { describe, it, expect } from 'vitest'
import {
  groupCtaState,
  groupLandingPath,
  groupStatusLine,
  isLastSeatedPlayer,
  parseGroupRoute,
  playersPickingASeat,
  selectGroupTable,
} from './group'

function slot(seatKey, displayName, user = null) {
  return { seatKey, queuePath: 'Player', displayName, user }
}

describe('parseGroupRoute', () => {
  it('matches the group path', () => {
    expect(parseGroupRoute('/group')).toBe(true)
    expect(parseGroupRoute('/group/')).toBe(true)
  })

  it('does not match other paths', () => {
    expect(parseGroupRoute('/')).toBe(false)
    expect(parseGroupRoute('/groups')).toBe(false)
    expect(parseGroupRoute('/room/ABC123')).toBe(false)
  })
})

describe('groupStatusLine', () => {
  it('names the roles still needed while the table is filling', () => {
    const table = {
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u1' }), slot('p-2', 'Player · 2')],
      formingGaps: [{ queuePath: 'Player', displayName: 'Player', assigned: 1, needed: 2 }],
    }

    expect(groupStatusLine(table)).toBe('Still need: Player')
  })

  it('reports readiness once every seat is taken', () => {
    const table = {
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u1' }), slot('p-2', 'Player · 2', { id: 'u2' })],
      formingGaps: [{ queuePath: 'Player', displayName: 'Player', assigned: 2, needed: 2 }],
    }

    expect(groupStatusLine(table)).toBe('2 of 2 seats · ready to start')
  })
})

describe('playersPickingASeat', () => {
  it('lists room members who hold no seat', () => {
    const room = { members: [{ id: 'u1' }, { id: 'u2' }, { id: 'u3' }] }
    const table = {
      seats: [{ seatKey: 'p-1', user: { id: 'u2' } }],
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u2' }), slot('p-2', 'Player · 2')],
    }

    expect(playersPickingASeat(room, table).map((m) => m.id)).toEqual(['u1', 'u3'])
  })
})

describe('groupCtaState', () => {
  const king = { id: 'u1', displayName: 'Alex' }

  it('invites an unseated player to claim a seat', () => {
    const table = { king, seats: [], seatSlots: [slot('p-1', 'Player · 1')] }

    expect(groupCtaState(table, 'u9')).toMatchObject({
      kind: 'claim',
      label: 'Claim a seat to join',
    })
  })

  it('tells a seated non-king who they are waiting on', () => {
    const table = {
      king,
      canStart: true,
      seats: [{ seatKey: 'p-2', user: { id: 'u9' } }],
      seatSlots: [slot('p-1', 'Player · 1', king), slot('p-2', 'Player · 2', { id: 'u9' })],
    }

    expect(groupCtaState(table, 'u9')).toMatchObject({
      kind: 'waiting',
      label: 'Waiting for Alex to start',
      hint: "You'll be taken in automatically",
    })
  })

  it('offers the start control to the king once the table can start', () => {
    const table = {
      king,
      canStart: true,
      seats: [{ seatKey: 'p-1', user: king }],
      seatSlots: [slot('p-1', 'Player · 1', king)],
    }

    expect(groupCtaState(table, 'u1')).toMatchObject({ kind: 'start', label: 'Start game' })
  })

  it('blocks the king with what is still missing', () => {
    const table = {
      king,
      canStart: false,
      seats: [{ seatKey: 'p-1', user: king }],
      seatSlots: [slot('p-1', 'Player · 1', king), slot('p-2', 'Player · 2')],
      formingGaps: [{ queuePath: 'Player', displayName: 'Player', assigned: 1, needed: 2 }],
    }

    expect(groupCtaState(table, 'u1')).toMatchObject({
      kind: 'blocked',
      label: 'Still need: Player',
    })
  })
})

describe('isLastSeatedPlayer', () => {
  it('is true when the player holds the only seat', () => {
    const table = { seats: [{ seatKey: 'p-1', user: { id: 'u1' } }], seatSlots: [] }
    expect(isLastSeatedPlayer(table, 'u1')).toBe(true)
  })

  it('is false when somebody else is still seated', () => {
    const table = {
      seats: [{ seatKey: 'p-1', user: { id: 'u1' } }, { seatKey: 'p-2', user: { id: 'u2' } }],
      seatSlots: [],
    }
    expect(isLastSeatedPlayer(table, 'u1')).toBe(false)
  })
})

describe('selectGroupTable', () => {
  it('prefers the table the player is seated at', () => {
    const room = {
      tables: [
        { id: 't1', createdAt: '2026-09-01T00:00:00Z', seats: [], seatSlots: [] },
        {
          id: 't2',
          createdAt: '2026-09-02T00:00:00Z',
          seats: [{ seatKey: 'p-1', user: { id: 'u1' } }],
          seatSlots: [],
        },
      ],
    }

    expect(selectGroupTable(room, 'u1')?.id).toBe('t2')
  })

  it('falls back to the newest table when the player holds no seat', () => {
    const room = {
      tables: [
        { id: 't1', createdAt: '2026-09-01T00:00:00Z', seats: [], seatSlots: [] },
        { id: 't2', createdAt: '2026-09-02T00:00:00Z', seats: [], seatSlots: [] },
      ],
    }

    expect(selectGroupTable(room, 'u1')?.id).toBe('t2')
  })

  it('is null when the room has no tables', () => {
    expect(selectGroupTable({ tables: [] }, 'u1')).toBeNull()
  })
})

describe('groupLandingPath', () => {
  it('sends an invite-link arrival to the group when the room holds one table', () => {
    expect(groupLandingPath({ tables: [{ id: 't1' }] })).toBe('/group')
  })

  it('leaves a room with no table alone', () => {
    expect(groupLandingPath({ tables: [] })).toBeNull()
  })

  it('leaves a multi-table room on the room surfaces', () => {
    expect(groupLandingPath({ tables: [{ id: 't1' }, { id: 't2' }] })).toBeNull()
  })
})
