import { describe, it, expect } from 'vitest'
import {
  enrichTableSeats,
  firstOpenSeatKey,
  formatGroupSeatCaption,
  groupSeatSlotsByTeam,
  groupSeatSlotsForDisplay,
  isPooledRoleGroup,
  mergeTableRecord,
  mySeatDisplayName,
  mySeatKeyOnTable,
  seatLabelInSection,
  seatSectionTitle,
  sectionTitleForSeat,
  tableShouldLeaveRoomList,
} from './tables'

describe('groupSeatSlotsByTeam', () => {
  it('puts fifo seats in ungrouped only', () => {
    const slots = [
      { seatKey: '1', displayName: '1' },
      { seatKey: '2', displayName: '2' },
    ]
    const { teams, ungrouped } = groupSeatSlotsByTeam(slots)
    expect(teams).toHaveLength(0)
    expect(ungrouped).toHaveLength(2)
  })

  it('groups team-prefixed seats separately', () => {
    const slots = [
      { seatKey: 'Team-1-DPS-1', displayName: 'DPS · 1', teamPrefix: 'Team-1' },
      { seatKey: 'Team-2-DPS-1', displayName: 'DPS · 1', teamPrefix: 'Team-2' },
    ]
    const { teams, ungrouped } = groupSeatSlotsByTeam(slots)
    expect(teams).toHaveLength(2)
    expect(ungrouped).toHaveLength(0)
  })
})

describe('groupSeatSlotsForDisplay', () => {
  it('groups composition roles into sections', () => {
    const layout = groupSeatSlotsForDisplay([
      { seatKey: 'ClueGiver-Red', displayName: 'Clue Giver · Red', queuePath: 'ClueGiver' },
      { seatKey: 'ClueGiver-Blue', displayName: 'Clue Giver · Blue', queuePath: 'ClueGiver' },
      { seatKey: 'Guesser-1', displayName: 'Guesser · 1', queuePath: 'Guesser' },
    ])
    expect(layout.kind).toBe('roles')
    expect(layout.roles).toHaveLength(2)
    expect(layout.roles[0][0]).toBe('Clue Giver')
  })
})

describe('seatSectionTitle', () => {
  it('takes the role off the seat label', () => {
    expect(seatSectionTitle([{ displayName: 'Clue Giver · Red' }])).toBe('Clue Giver')
  })

  it('falls back rather than heading a section with a seat number', () => {
    expect(seatSectionTitle([{ displayName: '1' }])).toBe('Player')
    expect(seatSectionTitle([{ displayName: '' }])).toBe('Player')
    expect(seatSectionTitle([{ displayName: '2' }], 'fifo')).toBe('fifo')
  })
})

describe('seatLabelInSection', () => {
  it('shortens clue giver seats to color only under section header', () => {
    expect(
      seatLabelInSection({ displayName: 'Clue Giver · Red' }, 'Clue Giver'),
    ).toBe('Red')
  })

  it('hides numbered guesser seats under section header', () => {
    expect(seatLabelInSection({ displayName: 'Guesser · 1' }, 'Guesser')).toBeNull()
    expect(seatLabelInSection({ displayName: 'Guesser · 2' }, 'Guesser')).toBeNull()
  })
})

describe('sectionTitleForSeat', () => {
  const wordHuntSlots = [
    { seatKey: 'x', displayName: 'Clue Giver · Red', queuePath: 'ClueGiver' },
    { seatKey: 'y', displayName: 'Guesser · 1', queuePath: 'Guesser' },
  ]

  it('finds role section title for a seat key', () => {
    expect(sectionTitleForSeat('x', wordHuntSlots)).toBe('Clue Giver')
    expect(sectionTitleForSeat('y', wordHuntSlots)).toBe('Guesser')
  })
})

describe('tableShouldLeaveRoomList', () => {
  it('removes started tables with no seated players', () => {
    expect(
      tableShouldLeaveRoomList({
        id: 't1',
        canStart: false,
        seats: [],
        seatSlots: [{ seatKey: '1', displayName: '1' }],
      }),
    ).toBe(true)
  })

  it('keeps forming tables with open seats', () => {
    expect(
      tableShouldLeaveRoomList({
        id: 't1',
        canStart: false,
        seats: [{ seatKey: '1', user: { id: 'u1' } }],
        seatSlots: [{ seatKey: '1', displayName: '1', user: { id: 'u1' } }],
      }),
    ).toBe(false)
  })

  // A private table is created with nobody seated, so "empty and cannot start" is the
  // opening state of every group, not a finished one. Occupancy cannot tell those
  // apart; status can (JQ-132).
  it('keeps a forming table that nobody has claimed a seat at yet', () => {
    expect(
      tableShouldLeaveRoomList({
        id: 't1',
        status: 'forming',
        canStart: false,
        seats: [],
        seatSlots: [
          { seatKey: '1', displayName: '1', user: null },
          { seatKey: '2', displayName: '2', user: null },
        ],
      }),
    ).toBe(false)
  })

  it('removes a table once it has started', () => {
    expect(
      tableShouldLeaveRoomList({
        id: 't1',
        status: 'started',
        canStart: true,
        seats: [{ seatKey: '1', user: { id: 'u1' } }],
        seatSlots: [{ seatKey: '1', displayName: '1', user: { id: 'u1' } }],
      }),
    ).toBe(true)
  })

  it('removes a discarded table', () => {
    expect(
      tableShouldLeaveRoomList({ id: 't1', status: 'discarded', canStart: false, seats: [], seatSlots: [] }),
    ).toBe(true)
  })
})

describe('pooled role helpers', () => {
  const guesserSlots = [
    { seatKey: 'Guesser-1', displayName: 'Guesser · 1', queuePath: 'Guesser' },
    { seatKey: 'Guesser-2', displayName: 'Guesser · 2', queuePath: 'Guesser' },
  ]
  const clueGiverSlots = [
    { seatKey: 'ClueGiver-Red', displayName: 'Clue Giver · Red', queuePath: 'ClueGiver' },
    { seatKey: 'ClueGiver-Blue', displayName: 'Clue Giver · Blue', queuePath: 'ClueGiver' },
    { seatKey: 'ClueGiver-Green', displayName: 'Clue Giver · Green', queuePath: 'ClueGiver' },
  ]

  it('detects pooled guesser groups', () => {
    expect(isPooledRoleGroup(guesserSlots)).toBe(true)
  })

  it('detects pooled clue giver groups', () => {
    expect(isPooledRoleGroup(clueGiverSlots)).toBe(true)
  })

  it('does not pool mixed or unpathed slots', () => {
    expect(isPooledRoleGroup([])).toBe(false)
    expect(isPooledRoleGroup([{ seatKey: '1', displayName: '1' }])).toBe(false)
    expect(
      isPooledRoleGroup([
        { seatKey: 'a', queuePath: 'ClueGiver' },
        { seatKey: 'b', queuePath: 'Guesser' },
      ]),
    ).toBe(false)
  })

  it('pools fifo duel seats without queue paths', () => {
    expect(
      isPooledRoleGroup([
        { seatKey: '1', displayName: '1' },
        { seatKey: '2', displayName: '2' },
      ]),
    ).toBe(true)
  })

  it('picks the first open seat in a pooled group', () => {
    const slots = [
      { seatKey: 'Guesser-1', displayName: 'Guesser · 1', user: { id: 'u1' } },
      { seatKey: 'Guesser-2', displayName: 'Guesser · 2', user: null },
    ]
    expect(firstOpenSeatKey(slots)).toBe('Guesser-2')
  })

  it('formats min/max captions', () => {
    expect(formatGroupSeatCaption(1, { minPlayers: 2, maxPlayers: 6 })).toBe('1/6 seated · need 2 to start')
    expect(formatGroupSeatCaption(2, { minPlayers: 2, maxPlayers: 6 })).toBe('2/6 seated · ready')
  })
})

describe('mergeTableRecord', () => {
  it('replaces seats when update includes an empty list', () => {
    const prev = {
      id: 't1',
      seats: [{ seatKey: '1', user: { id: 'u1' } }],
      seatSlots: [{ seatKey: '1', displayName: '1', user: { id: 'u1' } }],
    }
    const merged = mergeTableRecord(prev, { id: 't1', seats: [], seatSlots: [{ seatKey: '1', displayName: '1' }] })
    expect(merged.seats).toEqual([])
    expect(merged.seatSlots[0].user).toBeUndefined()
  })
})

describe('enrichTableSeats', () => {
  it('fills slot users from table.seats when missing on slots', () => {
    const enriched = enrichTableSeats({
      seats: [{ seatKey: '1', user: { id: 'u1', displayName: 'Pat' } }],
      seatSlots: [{ seatKey: '1', displayName: 'Seat 1' }],
    })
    expect(enriched.seatSlots[0].user?.displayName).toBe('Pat')
  })
})

describe('mySeatKeyOnTable', () => {
  it('finds seat from seats array', () => {
    const key = mySeatKeyOnTable(
      { seats: [{ seatKey: 'a', user: { id: 'u1' } }] },
      'u1',
    )
    expect(key).toBe('a')
  })

  it('falls back to seatSlots when seats missing', () => {
    const key = mySeatKeyOnTable(
      { seatSlots: [{ seatKey: 'b', user: { id: 'u2' } }] },
      'u2',
    )
    expect(key).toBe('b')
  })
})

describe('mySeatDisplayName', () => {
  it('returns shortened seat label for role sections', () => {
    const name = mySeatDisplayName(
      {
        seats: [{ seatKey: 'x', user: { id: 'u1' } }],
        seatSlots: [
          { seatKey: 'x', displayName: 'Clue Giver · Red', queuePath: 'ClueGiver' },
          { seatKey: 'y', displayName: 'Guesser · 1', queuePath: 'Guesser' },
        ],
      },
      'u1',
    )
    expect(name).toBe('Clue Giver · Red')
  })

  it('returns role name only for numbered guesser seats', () => {
    const name = mySeatDisplayName(
      {
        seats: [{ seatKey: 'y', user: { id: 'u1' } }],
        seatSlots: [
          { seatKey: 'x', displayName: 'Clue Giver · Red', queuePath: 'ClueGiver' },
          { seatKey: 'y', displayName: 'Guesser · 2', queuePath: 'Guesser' },
        ],
      },
      'u1',
    )
    expect(name).toBe('Guesser')
  })
})
