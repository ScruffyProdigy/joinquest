import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import GroupPage from './GroupPage'

const mutations = {
  sitAtTable: vi.fn(),
  leaveTable: vi.fn(),
  discardTable: vi.fn(),
  startTable: vi.fn(),
}

vi.mock('../../lib/tables', async (importOriginal) => ({
  ...(await importOriginal()),
  sitAtTable: (...args) => mutations.sitAtTable(...args),
  leaveTable: (...args) => mutations.leaveTable(...args),
  discardTable: (...args) => mutations.discardTable(...args),
  startTable: (...args) => mutations.startTable(...args),
}))

const navigateTo = vi.fn()
vi.mock('../../lib/usePathname', () => ({
  navigateTo: (...args) => navigateTo(...args),
  usePathname: () => '/group',
}))

let currentUser = { id: 'u1', displayName: 'Pat' }
vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => ({ user: currentUser, loading: false }),
}))

let currentRoom = null
const refreshRoom = vi.fn()
vi.mock('../rooms/ActiveRoomProvider', () => ({
  useActiveRoom: () => ({ room: currentRoom, refresh: refreshRoom, loading: false }),
}))

const king = { id: 'u1', displayName: 'Alex' }

function makeRoom(table) {
  return {
    id: 'room-1',
    inviteCode: 'ABC123',
    joinUrl: 'https://joinquest.cc/room/ABC123',
    members: [{ id: 'u1', displayName: 'Pat' }, { id: 'u7', displayName: 'Sam' }],
    tables: [table],
  }
}

function makeTable(overrides = {}) {
  return {
    id: 'table-1',
    createdAt: '2026-09-01T00:00:00Z',
    game: { name: 'Word Hunt', accentColor: '#f2b134' },
    mode: { displayName: 'Arena', queuePaths: [{ queuePath: 'Player', minPlayers: 2, maxPlayers: 2 }] },
    king,
    canStart: false,
    seats: [],
    seatSlots: [
      { seatKey: 'p-1', queuePath: 'Player', displayName: 'Player · 1', user: null },
      { seatKey: 'p-2', queuePath: 'Player', displayName: 'Player · 2', user: null },
    ],
    formingGaps: [{ queuePath: 'Player', displayName: 'Player', assigned: 0, needed: 2 }],
    lookForGroupOptions: [],
    regroupRoster: [],
    ...overrides,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  currentUser = { id: 'u1', displayName: 'Pat' }
  currentRoom = makeRoom(makeTable())
})

describe('GroupPage', () => {
  it('renders the group heading and what the table still needs', () => {
    render(<GroupPage />)

    expect(screen.getByRole('heading', { name: 'Your Group' })).toBeInTheDocument()
    expect(screen.getByText('Still need: Player')).toBeInTheDocument()
  })

  it('lists room members who hold no seat under Picking a seat', () => {
    render(<GroupPage />)

    const section = screen.getByRole('region', { name: /picking a seat/i })
    expect(section).toHaveTextContent('Sam')
  })

  it('claims a seat through sitAtTable', async () => {
    const user = userEvent.setup()
    render(<GroupPage />)

    await user.click(screen.getAllByRole('button', { name: /claim/i })[0])

    expect(mutations.sitAtTable).toHaveBeenCalledWith('table-1', 'p-1')
  })

  it('prompts an unseated player to claim a seat', () => {
    render(<GroupPage />)

    expect(screen.getByText('Claim a seat to join')).toBeInTheDocument()
  })

  it('names who a seated player is waiting on, without saying king', () => {
    currentUser = { id: 'u9', displayName: 'Jo' }
    currentRoom = makeRoom(
      makeTable({
        canStart: true,
        seats: [{ seatKey: 'p-2', user: { id: 'u9' } }],
        seatSlots: [
          { seatKey: 'p-1', queuePath: 'Player', displayName: 'Player · 1', user: king },
          { seatKey: 'p-2', queuePath: 'Player', displayName: 'Player · 2', user: { id: 'u9' } },
        ],
        formingGaps: [],
      }),
    )
    render(<GroupPage />)

    expect(screen.getByText('Waiting for Alex to start')).toBeInTheDocument()
    expect(screen.getByText("You'll be taken in automatically")).toBeInTheDocument()
    expect(screen.queryByText(/\bking\b/i)).not.toBeInTheDocument()
  })

  it('starts the game for the king once the table can start', async () => {
    const user = userEvent.setup()
    mutations.startTable.mockResolvedValue({ joinUrl: null })
    currentRoom = makeRoom(
      makeTable({
        canStart: true,
        seats: [{ seatKey: 'p-1', user: king }],
        seatSlots: [
          { seatKey: 'p-1', queuePath: 'Player', displayName: 'Player · 1', user: king },
          { seatKey: 'p-2', queuePath: 'Player', displayName: 'Player · 2', user: { id: 'u7' } },
        ],
        formingGaps: [],
      }),
    )
    render(<GroupPage />)

    await user.click(screen.getByRole('button', { name: 'Start game' }))

    expect(mutations.startTable).toHaveBeenCalledWith('table-1')
  })

  it('leaves the table when the player navigates back', async () => {
    const user = userEvent.setup()
    mutations.leaveTable.mockResolvedValue(true)
    currentRoom = makeRoom(
      makeTable({
        seats: [{ seatKey: 'p-1', user: { id: 'u1' } }, { seatKey: 'p-2', user: { id: 'u7' } }],
        seatSlots: [
          { seatKey: 'p-1', queuePath: 'Player', displayName: 'Player · 1', user: { id: 'u1' } },
          { seatKey: 'p-2', queuePath: 'Player', displayName: 'Player · 2', user: { id: 'u7' } },
        ],
      }),
    )
    render(<GroupPage />)

    await user.click(screen.getByRole('button', { name: /leave group/i }))

    await waitFor(() => expect(mutations.leaveTable).toHaveBeenCalledWith('table-1'))
    expect(mutations.discardTable).not.toHaveBeenCalled()
    expect(navigateTo).toHaveBeenCalledWith('/', { replace: true })
  })

  it('discards the table when the last seated player leaves', async () => {
    const user = userEvent.setup()
    mutations.leaveTable.mockResolvedValue(true)
    mutations.discardTable.mockResolvedValue(true)
    currentRoom = makeRoom(
      makeTable({
        seats: [{ seatKey: 'p-1', user: { id: 'u1' } }],
        seatSlots: [
          { seatKey: 'p-1', queuePath: 'Player', displayName: 'Player · 1', user: { id: 'u1' } },
          { seatKey: 'p-2', queuePath: 'Player', displayName: 'Player · 2', user: null },
        ],
      }),
    )
    render(<GroupPage />)

    await user.click(screen.getByRole('button', { name: /leave group/i }))

    await waitFor(() => expect(mutations.discardTable).toHaveBeenCalledWith('table-1'))
  })

  it('does not release the seat on reload, only on deliberate navigation', () => {
    const addEventListener = vi.spyOn(window, 'addEventListener')

    render(<GroupPage />)

    const listened = addEventListener.mock.calls.map(([event]) => event)
    expect(listened).not.toContain('beforeunload')
    expect(listened).not.toContain('unload')
    expect(listened).not.toContain('pagehide')
  })

  it('shows the share link so friends can join', () => {
    render(<GroupPage />)

    expect(screen.getByRole('region', { name: /invite friends/i })).toBeInTheDocument()
    expect(screen.queryByText(/room code/i)).not.toBeInTheDocument()
  })
})
