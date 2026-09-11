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

  it('names the role from the open seats when the API reports no forming gaps', () => {
    // A private table nobody is queueing for has no formingGaps, and a mode that names
    // no roles numbers its seats — the header still has to say what is missing.
    const table = {
      seatSlots: [
        { seatKey: 'p-1', queuePath: 'Player', displayName: '1', user: { id: 'u1' } },
        { seatKey: 'p-2', queuePath: 'Player', displayName: '2', user: null },
      ],
      formingGaps: [],
    }

    expect(groupStatusLine(table)).toBe('Still need: Player')
  })

  it('never counts seats at the player when one is still open', () => {
    const table = {
      seatSlots: [slot('p-1', 'Sheriff · 1'), slot('p-2', 'Outlaw · 1')],
      formingGaps: [],
    }

    expect(groupStatusLine(table)).toBe('Still need: Sheriff, Outlaw')
  })

  it('reports readiness once every seat is taken', () => {
    const table = {
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u1' }), slot('p-2', 'Player · 2', { id: 'u2' })],
      // `needed` is the remainder the API computed, not the requirement: a filled path
      // reports zero. (It used to read 2 here, which no live table can produce.)
      formingGaps: [{ queuePath: 'Player', displayName: 'Player', assigned: 2, needed: 0 }],
    }

    expect(groupStatusLine(table)).toBe('2 of 2 seats · ready to start')
  })

  it('still names a role that needs exactly as many as it already has', () => {
    // Guards the reading above: `needed > assigned` would call this path satisfied.
    const table = {
      seatSlots: [
        slot('p-1', 'Guesser · 1', { id: 'u1' }),
        slot('p-2', 'Guesser · 2', { id: 'u2' }),
        slot('p-3', 'Guesser · 3'),
        slot('p-4', 'Guesser · 4'),
      ],
      formingGaps: [{ queuePath: 'Guesser', displayName: 'Guesser', assigned: 2, needed: 2 }],
    }

    expect(groupStatusLine(table)).toBe('Still need: Guesser')
  })
})

describe('playersPickingASeat', () => {
  function entryIds(entries) {
    return entries.map((entry) => entry.user.id)
  }

  it('lists room members who hold no seat', () => {
    const room = { members: [{ user: { id: 'u1' }, disconnected: false }, { user: { id: 'u2' }, disconnected: false }, { user: { id: 'u3' }, disconnected: false }] }
    const table = {
      seats: [{ seatKey: 'p-1', user: { id: 'u2' } }],
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u2' }), slot('p-2', 'Player · 2')],
    }

    expect(entryIds(playersPickingASeat(room, table))).toEqual(['u1', 'u3'])
    expect(playersPickingASeat(room, table).map((entry) => entry.status)).toEqual(['here', 'here'])
  })

  it('leaves a seated player off the card even when their regroup answer is pending', () => {
    const room = { members: [{ user: { id: 'u1' }, disconnected: false }] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u1' })],
      regroupRoster: [{ user: { id: 'u1' }, role: 'p-1', regroup: 'PENDING' }],
    }

    expect(playersPickingASeat(room, table, 'u1')).toEqual([])
  })

  it('shows a pending previous-match player as awaiting', () => {
    const room = { members: [{ user: { id: 'u1' }, disconnected: false }] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u1' }), slot('p-2', 'Player · 2')],
      regroupRoster: [{ user: { id: 'u2' }, role: 'p-2', regroup: 'PENDING' }],
    }

    expect(playersPickingASeat(room, table, 'u1')).toEqual([
      { user: { id: 'u2' }, status: 'awaiting', away: false },
    ])
  })

  it('keeps a pending room member awaiting rather than here', () => {
    // A room-table group never leaves the room, so membership cannot stand in for
    // "they are back" the way the prototype's arrival does. The regroup answer is the
    // only thing that actually says whether they have decided.
    const room = { members: [{ user: { id: 'u1' }, disconnected: false }, { user: { id: 'u2' }, disconnected: false }] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u1' }), slot('p-2', 'Player · 2')],
      regroupRoster: [{ user: { id: 'u2' }, role: 'p-2', regroup: 'PENDING' }],
    }

    expect(playersPickingASeat(room, table, 'u1')).toEqual([
      { user: { id: 'u2' }, status: 'awaiting', away: false },
    ])
  })

  it('never shows the viewer as awaiting — they are demonstrably back', () => {
    const room = { members: [{ user: { id: 'u1' }, disconnected: false }, { user: { id: 'u2' }, disconnected: false }] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1'), slot('p-2', 'Player · 2', { id: 'u2' })],
      regroupRoster: [{ user: { id: 'u1' }, role: 'p-1', regroup: 'PENDING' }],
    }

    expect(playersPickingASeat(room, table, 'u1')).toEqual([
      { user: { id: 'u1' }, status: 'here', away: false },
    ])
  })

  it('marks a declined previous-match player out', () => {
    const room = { members: [] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1')],
      regroupRoster: [{ user: { id: 'u2' }, role: 'p-1', regroup: 'OUT' }],
    }

    expect(playersPickingASeat(room, table, 'u1')).toEqual([
      { user: { id: 'u2' }, status: 'out', away: false },
    ])
  })

  // JQ-265. The reading belongs to the room, and the group screen is a presentation over
  // the room, so it has to survive the trip rather than be recomputed here. It arrives as
  // `disconnected` (what the API observed) and lands as `away` (what the card shows).
  it('carries a member disconnected reading through to their entry as away', () => {
    const room = {
      members: [
        { user: { id: 'u1' }, disconnected: false },
        { user: { id: 'u2' }, disconnected: true },
      ],
    }
    const table = { seatSlots: [slot('p-1', 'Player · 1')], regroupRoster: [] }

    const byId = new Map(playersPickingASeat(room, table, 'u1').map((e) => [e.user.id, e]))
    expect(byId.get('u1').away).toBe(false)
    expect(byId.get('u2').away).toBe(true)
  })

  // A player can be awaiting a regroup answer and away at once, so the two readings have
  // to be independent fields. Collapsing away into `status` would have to drop one.
  it('keeps away independent of an awaiting status', () => {
    const room = { members: [{ user: { id: 'u2' }, disconnected: true }] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1', { id: 'u1' }), slot('p-2', 'Player · 2')],
      regroupRoster: [{ user: { id: 'u2' }, role: 'p-2', regroup: 'PENDING' }],
    }

    expect(playersPickingASeat(room, table, 'u1')).toEqual([
      { user: { id: 'u2' }, status: 'awaiting', away: true },
    ])
  })

  // Someone on the regroup roster who is no longer a room member has left outright, which
  // is a stronger statement than away — and that roster records answers, not sockets, so
  // there is no presence there to read in the first place.
  it('does not invent an away reading for a non-member on the regroup roster', () => {
    const room = { members: [] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1')],
      regroupRoster: [{ user: { id: 'u9' }, role: 'p-1', regroup: 'PENDING' }],
    }

    expect(playersPickingASeat(room, table, 'u1')).toEqual([
      { user: { id: 'u9' }, status: 'awaiting', away: false },
    ])
  })

  it('lists each player once when they are both a room member and on the roster', () => {
    const room = { members: [{ user: { id: 'u1' }, disconnected: false }, { user: { id: 'u2' }, disconnected: false }] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1'), slot('p-2', 'Player · 2')],
      regroupRoster: [{ user: { id: 'u2' }, role: 'p-2', regroup: 'PENDING' }],
    }

    expect(entryIds(playersPickingASeat(room, table, 'u1'))).toEqual(['u1', 'u2'])
  })

  it('lists pending players in a mode with no seat template', () => {
    // FIFO: no seatSlots to pair against. Nothing here keys on seatKey, so the roster
    // still lists.
    const room = { members: [] }
    const table = {
      seatSlots: [],
      regroupRoster: [
        { user: { id: 'u2' }, role: null, regroup: 'PENDING' },
        { user: { id: 'u3' }, role: null, regroup: 'PENDING' },
      ],
    }

    expect(entryIds(playersPickingASeat(room, table, 'u1'))).toEqual(['u2', 'u3'])
  })

  it('drops nobody when there are more pending players than seats', () => {
    const room = { members: [] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1'), slot('p-2', 'Player · 2')],
      regroupRoster: [
        { user: { id: 'u2' }, role: 'p-1', regroup: 'PENDING' },
        { user: { id: 'u3' }, role: 'p-1', regroup: 'PENDING' },
        { user: { id: 'u4' }, role: 'p-2', regroup: 'PENDING' },
      ],
    }

    expect(entryIds(playersPickingASeat(room, table, 'u1'))).toEqual(['u2', 'u3', 'u4'])
  })

  it('orders the viewer, then members, then awaiting, then out', () => {
    const room = { members: [{ user: { id: 'u5' }, disconnected: false }, { user: { id: 'u1' }, disconnected: false }] }
    const table = {
      seatSlots: [slot('p-1', 'Player · 1'), slot('p-2', 'Player · 2')],
      regroupRoster: [
        { user: { id: 'u4' }, role: 'p-2', regroup: 'OUT' },
        { user: { id: 'u3' }, role: 'p-1', regroup: 'PENDING' },
      ],
    }

    expect(entryIds(playersPickingASeat(room, table, 'u1'))).toEqual(['u1', 'u5', 'u3', 'u4'])
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

  // A table that is playable but not full, with no queue to ask. Both king branches
  // still live here — a full table no longer reaches either of them.
  function startableTable(overrides = {}) {
    return {
      king,
      canStart: true,
      seats: [{ seatKey: 'p-1', user: king }, { seatKey: 'p-2', user: { id: 'u9' } }],
      seatSlots: [
        slot('p-1', 'Player · 1', king),
        slot('p-2', 'Player · 2', { id: 'u9' }),
        slot('p-3', 'Player · 3'),
      ],
      lookForGroupOptions: [],
      formingGaps: [],
      ...overrides,
    }
  }

  it('tells a seated non-king who they are waiting on', () => {
    expect(groupCtaState(startableTable(), 'u9')).toMatchObject({
      kind: 'waiting',
      label: 'Waiting for Alex to start',
      hint: "You'll be taken in automatically",
    })
  })

  it('offers the start control to the king once the table can start', () => {
    expect(groupCtaState(startableTable(), 'u1')).toMatchObject({
      kind: 'start',
      label: 'Start game',
    })
  })

  it('offers the king the way to ask for the rest of the match', () => {
    const table = startableTable({
      canStart: false,
      lookForGroupOptions: [{ queueId: 'q-1', queueName: 'Ranked', visible: true, enabled: true }],
    })

    expect(groupCtaState(table, 'u1')).toMatchObject({
      kind: 'find',
      label: 'Find us a match',
      queueId: 'q-1',
      startLabel: null,
    })
  })

  it('offers nobody else that control — filling the table ends the wait for everyone', () => {
    const table = startableTable({
      canStart: false,
      lookForGroupOptions: [{ queueId: 'q-1', queueName: 'Ranked', visible: true, enabled: true }],
    })

    expect(groupCtaState(table, 'u9')).toMatchObject({
      kind: 'waiting',
      label: 'Waiting for Alex to start',
      hint: "You'll be taken in automatically",
    })
  })

  it('keeps playing short beside it once the table is past its minimum', () => {
    const table = startableTable({
      lookForGroupOptions: [{ queueId: 'q-1', queueName: 'Ranked', visible: true, enabled: true }],
    })

    expect(groupCtaState(table, 'u1')).toMatchObject({ kind: 'find', startLabel: 'Start game' })
  })

  it('ignores a queue the table may not ask right now', () => {
    const table = startableTable({
      lookForGroupOptions: [{ queueId: 'q-1', queueName: 'Ranked', visible: true, enabled: false }],
    })

    expect(groupCtaState(table, 'u1')).toMatchObject({ kind: 'start' })
    expect(groupCtaState(table, 'u9')).toMatchObject({ kind: 'waiting' })
  })

  it('shows what is still filling to everyone, and offers Stop only to the king', () => {
    const table = startableTable({
      backfillActive: true,
      formingGaps: [
        { queuePath: 'Attacker', displayName: 'Attacker', assigned: 2, needed: 1 },
        { queuePath: 'Defender', displayName: 'Defender', assigned: 1, needed: 2 },
      ],
    })

    expect(groupCtaState(table, 'u1')).toMatchObject({
      kind: 'filling',
      label: 'Finding your match',
      detail: 'Need 1 Attacker, 2 Defender',
      cancelLabel: 'Stop',
    })
    expect(groupCtaState(table, 'u9')).toMatchObject({
      kind: 'filling',
      detail: 'Need 1 Attacker, 2 Defender',
      cancelLabel: null,
    })
  })

  it('counts rather than names the gap for a mode with no roles of its own', () => {
    const table = startableTable({
      backfillActive: true,
      formingGaps: [{ queuePath: '', displayName: '', assigned: 3, needed: 3 }],
    })

    expect(groupCtaState(table, 'u1')).toMatchObject({ detail: 'Need 3 more' })
  })

  it('still waits for the king once every seat is taken — full is ready, not decided', () => {
    const table = startableTable({
      seatSlots: [
        slot('p-1', 'Player · 1', king),
        slot('p-2', 'Player · 2', { id: 'u9' }),
        slot('p-3', 'Player · 3', { id: 'u7' }),
      ],
      lookForGroupOptions: [],
    })

    expect(groupCtaState(table, 'u1')).toMatchObject({ kind: 'start', label: 'Start game' })
    expect(groupCtaState(table, 'u9')).toMatchObject({ kind: 'waiting' })
  })

  it('blocks the king with what is still missing', () => {
    const table = {
      king,
      canStart: false,
      seats: [{ seatKey: 'p-1', user: king }],
      seatSlots: [slot('p-1', 'Player · 1', king), slot('p-2', 'Player · 2')],
      lookForGroupOptions: [],
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
