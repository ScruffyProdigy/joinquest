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

const fetchModeQueueOptions = vi.fn()
vi.mock('../../lib/games', async (importOriginal) => ({
  ...(await importOriginal()),
  fetchModeQueueOptions: (...args) => fetchModeQueueOptions(...args),
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
    game: { id: 'game-1', name: 'Word Hunt', accentColor: '#f2b134' },
    mode: {
      id: 'mode-1',
      displayName: 'Arena',
      queuePaths: [{ queuePath: 'Player', minPlayers: 2, maxPlayers: 2 }],
      preQueueGroups: [],
    },
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

  it('lists the viewer alone under Picking a seat before they claim', () => {
    currentRoom = { ...makeRoom(makeTable()), members: [{ id: 'u1', displayName: 'Pat' }] }
    render(<GroupPage />)

    const section = screen.getByRole('region', { name: /picking a seat/i })
    expect(section).toHaveTextContent('You')
  })

  it('lists a previous match player who has not answered as awaiting', () => {
    currentRoom = makeRoom(
      makeTable({
        regroupRoster: [{ user: { id: 'u9', displayName: 'Rae' }, role: 'p-2', regroup: 'PENDING' }],
      }),
    )
    render(<GroupPage />)

    const section = screen.getByRole('region', { name: /picking a seat/i })
    expect(section).toHaveTextContent('Rae')
    expect(section).toHaveTextContent('Awaiting')
  })

  it('never shows the viewer as awaiting, whatever their regroup answer says', () => {
    currentRoom = makeRoom(
      makeTable({
        regroupRoster: [{ user: { id: 'u1', displayName: 'Pat' }, role: 'p-1', regroup: 'PENDING' }],
      }),
    )
    render(<GroupPage />)

    const you = screen.getByRole('region', { name: /picking a seat/i }).querySelector('li')
    expect(you).toHaveTextContent('You')
    expect(you).not.toHaveTextContent('Awaiting')
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

  it('invites with a QR and one Share Link, not a row of ways to send a URL', async () => {
    render(<GroupPage />)

    const invite = screen.getByRole('region', { name: /invite friends/i })
    expect(await screen.findByRole('img', { name: /qr code to join/i })).toBeInTheDocument()
    const labels = [...invite.querySelectorAll('button')].map((button) =>
      button.textContent.trim(),
    )
    expect(labels).toEqual(['Share Link'])
  })

  it('names the role on the seat row even when the seats are only numbered', () => {
    currentRoom = makeRoom(
      makeTable({
        seatSlots: [
          { seatKey: 'p-1', queuePath: 'Player', displayName: '1', user: null },
          { seatKey: 'p-2', queuePath: 'Player', displayName: '2', user: null },
        ],
      }),
    )
    render(<GroupPage />)

    const players = screen.getByRole('region', { name: 'Players' })
    expect(players).toHaveTextContent('Player × 2')
    expect(players).not.toHaveTextContent('1 × 2')
  })

  // The rejoin surface is a destination, not a trap: a player who would rather go back to
  // matchmaking must be able to say so here, in words (JQ-232).
  it('offers a named way back to matchmaking', () => {
    render(<GroupPage />)

    expect(screen.getByRole('link', { name: 'Find something new' })).toHaveAttribute('href', '/')
  })

  describe('a mode with pre-queue options', () => {
    function tableWithOptions(overrides = {}) {
      return makeTable({
        mode: {
          id: 'mode-1',
          displayName: 'Duel',
          queuePaths: [{ queuePath: 'Player', minPlayers: 2, maxPlayers: 2 }],
          preQueueGroups: [
            { key: 'helpers', kind: 'LOADOUT', label: 'Choose your two helpers', min: 2, max: 2 },
          ],
        },
        ...overrides,
      })
    }

    const roster = {
      available: true,
      unavailableReason: null,
      groups: [
        {
          key: 'helpers',
          choices: [
            { id: 'ferrus', label: 'Ferrus', description: null, locked: false },
            { id: 'tempered', label: 'Tempered', description: null, locked: false },
          ],
        },
      ],
    }

    it('opens the picker instead of seating the player straight away', async () => {
      const user = userEvent.setup()
      fetchModeQueueOptions.mockResolvedValue(roster)
      currentRoom = makeRoom(tableWithOptions())
      render(<GroupPage />)

      await user.click(screen.getAllByRole('button', { name: /claim/i })[0])

      expect(await screen.findByRole('dialog')).toHaveTextContent('Choose your two helpers')
      expect(mutations.sitAtTable).not.toHaveBeenCalled()
      expect(fetchModeQueueOptions).toHaveBeenCalledWith('game-1', 'mode-1', 'u1')
    })

    it('passes the picks through to sitAtTable', async () => {
      const user = userEvent.setup()
      fetchModeQueueOptions.mockResolvedValue(roster)
      currentRoom = makeRoom(tableWithOptions())
      render(<GroupPage />)

      await user.click(screen.getAllByRole('button', { name: /claim/i })[0])
      await screen.findByRole('dialog')
      await user.click(screen.getByRole('button', { name: 'Ferrus' }))
      await user.click(screen.getByRole('button', { name: 'Tempered' }))
      await user.click(screen.getByRole('button', { name: 'Take this seat' }))

      await waitFor(() =>
        expect(mutations.sitAtTable).toHaveBeenCalledWith('table-1', 'p-1', [
          { groupKey: 'helpers', optionIds: ['ferrus', 'tempered'] },
        ]),
      )
    })

    // Whatever the group brought last round, the sheet opens blank. Pre-selecting last
    // round's character pre-empts the re-choosing just as firmly as pre-seating does.
    it('pre-selects nothing, so the seat cannot be claimed without answering', async () => {
      const user = userEvent.setup()
      fetchModeQueueOptions.mockResolvedValue(roster)
      currentRoom = makeRoom(
        tableWithOptions({
          regroupRoster: [{ user: { id: 'u1', displayName: 'Pat' }, role: 'p-1', regroup: 'PENDING' }],
        }),
      )
      render(<GroupPage />)

      await user.click(screen.getAllByRole('button', { name: /claim/i })[0])
      await screen.findByRole('dialog')

      expect(screen.getByRole('button', { name: 'Ferrus' })).toHaveAttribute('aria-pressed', 'false')
      expect(screen.getByRole('button', { name: 'Take this seat' })).toBeDisabled()
    })

    // The roster comes only from the game, so an unreachable game must say so rather than
    // seating the player with no picks at all.
    it('says so when the roster cannot be reached', async () => {
      const user = userEvent.setup()
      fetchModeQueueOptions.mockRejectedValue(new Error('Word Hunt is not answering.'))
      currentRoom = makeRoom(tableWithOptions())
      render(<GroupPage />)

      await user.click(screen.getAllByRole('button', { name: /claim/i })[0])

      expect(await screen.findByRole('dialog')).toHaveTextContent('Word Hunt is not answering.')
      expect(screen.getByRole('button', { name: 'Take this seat' })).toBeDisabled()
    })
  })
})
