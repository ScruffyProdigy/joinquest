import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import GroupPage from './GroupPage'

const mutations = {
  sitAtTable: vi.fn(),
  leaveTable: vi.fn(),
  discardTable: vi.fn(),
  startTable: vi.fn(),
  startTableBackfill: vi.fn(),
  cancelTableBackfill: vi.fn(),
}

vi.mock('../../lib/tables', async (importOriginal) => ({
  ...(await importOriginal()),
  sitAtTable: (...args) => mutations.sitAtTable(...args),
  leaveTable: (...args) => mutations.leaveTable(...args),
  discardTable: (...args) => mutations.discardTable(...args),
  startTable: (...args) => mutations.startTable(...args),
  startTableBackfill: (...args) => mutations.startTableBackfill(...args),
  cancelTableBackfill: (...args) => mutations.cancelTableBackfill(...args),
}))

const fetchModeQueueOptions = vi.fn()
vi.mock('../../lib/games', async (importOriginal) => ({
  ...(await importOriginal()),
  fetchModeQueueOptions: (...args) => fetchModeQueueOptions(...args),
}))

const navigateToLaunchUrl = vi.fn()
vi.mock('../../lib/launch', async (importOriginal) => ({
  ...(await importOriginal()),
  navigateToLaunchUrl: (...args) => navigateToLaunchUrl(...args),
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
    members: [
      { user: { id: 'u1', displayName: 'Pat' }, disconnected: false },
      { user: { id: 'u7', displayName: 'Sam' }, disconnected: false },
    ],
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
    currentRoom = { ...makeRoom(makeTable()), members: [{ user: { id: 'u1', displayName: 'Pat' }, disconnected: false }] }
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

  // Playable but not full, with no queue to ask: the only shape that still reaches the
  // king's manual Start. A full table starts itself now (JQ-137).
  function startableTable(overrides = {}) {
    return makeTable({
      canStart: true,
      seats: [{ seatKey: 'p-1', user: king }, { seatKey: 'p-2', user: { id: 'u9' } }],
      seatSlots: [
        { seatKey: 'p-1', queuePath: 'Player', displayName: 'Player · 1', user: king },
        { seatKey: 'p-2', queuePath: 'Player', displayName: 'Player · 2', user: { id: 'u9' } },
        { seatKey: 'p-3', queuePath: 'Player', displayName: 'Player · 3', user: null },
      ],
      formingGaps: [],
      ...overrides,
    })
  }

  it('names who a seated player is waiting on, without saying king', () => {
    currentUser = { id: 'u9', displayName: 'Jo' }
    currentRoom = makeRoom(startableTable())
    render(<GroupPage />)

    expect(screen.getByText('Waiting for Alex to start')).toBeInTheDocument()
    expect(screen.getByText("You'll be taken in automatically")).toBeInTheDocument()
    expect(screen.queryByText(/\bking\b/i)).not.toBeInTheDocument()
  })

  it('starts the game for the king once the table can start', async () => {
    const user = userEvent.setup()
    mutations.startTable.mockResolvedValue({ joinUrl: null })
    currentRoom = makeRoom(startableTable())
    render(<GroupPage />)

    await user.click(screen.getByRole('button', { name: 'Start game' }))

    expect(mutations.startTable).toHaveBeenCalledWith('table-1')
  })

  describe('asking for the rest of the match', () => {
    const withQueue = (overrides = {}) =>
      startableTable({
        canStart: false,
        lookForGroupOptions: [
          { queueId: 'q-1', queueName: 'Ranked', visible: true, enabled: true },
        ],
        ...overrides,
      })

    it('lets the king ask, and says the group comes too', async () => {
      const user = userEvent.setup()
      mutations.startTableBackfill.mockResolvedValue({ queued: true })
      currentRoom = makeRoom(withQueue())
      render(<GroupPage />)

      expect(
        screen.getByText("Your group stays together — we'll fill the rest."),
      ).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: 'Find us a match' }))

      expect(mutations.startTableBackfill).toHaveBeenCalledWith('table-1', 'q-1')
      await waitFor(() => expect(refreshRoom).toHaveBeenCalled())
    })

    it('offers a seated friend no such button — filling the table ends the wait for everyone', () => {
      currentUser = { id: 'u9', displayName: 'Jo' }
      currentRoom = makeRoom(withQueue())
      render(<GroupPage />)

      expect(screen.queryByRole('button', { name: 'Find us a match' })).not.toBeInTheDocument()
      expect(screen.getByText('Waiting for Alex to start')).toBeInTheDocument()
    })

    it('never sends a player elsewhere on success — the seat push takes everyone in together', async () => {
      const user = userEvent.setup()
      mutations.startTableBackfill.mockResolvedValue({ queued: true, joinUrl: null })
      currentRoom = makeRoom(withQueue())
      render(<GroupPage />)

      await user.click(screen.getByRole('button', { name: 'Find us a match' }))

      expect(navigateTo).not.toHaveBeenCalled()
    })

    it('shows what is still filling, and lets the king stop it', async () => {
      const user = userEvent.setup()
      mutations.cancelTableBackfill.mockResolvedValue(true)
      currentRoom = makeRoom(
        withQueue({
          backfillActive: true,
          formingGaps: [
            { queuePath: 'Attacker', displayName: 'Attacker', assigned: 1, needed: 1 },
          ],
        }),
      )
      render(<GroupPage />)

      expect(screen.getByText('Finding your match')).toBeInTheDocument()
      expect(screen.getByText(/Need 1 Attacker/)).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Find us a match' })).not.toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: 'Stop' }))

      expect(mutations.cancelTableBackfill).toHaveBeenCalledWith('table-1')
    })

    it('tells a seated friend it is filling without offering them Stop', () => {
      currentUser = { id: 'u9', displayName: 'Jo' }
      currentRoom = makeRoom(
        withQueue({
          backfillActive: true,
          formingGaps: [
            { queuePath: 'Attacker', displayName: 'Attacker', assigned: 1, needed: 1 },
          ],
        }),
      )
      render(<GroupPage />)

      expect(screen.getByText('Finding your match')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Stop' })).not.toBeInTheDocument()
    })

    it('leaves a full table waiting on the king rather than starting it for them', async () => {
      const user = userEvent.setup()
      mutations.startTable.mockResolvedValue({ joinUrl: null })
      currentRoom = makeRoom(
        startableTable({
          seatSlots: [
            { seatKey: 'p-1', queuePath: 'Player', displayName: 'Player · 1', user: king },
            { seatKey: 'p-2', queuePath: 'Player', displayName: 'Player · 2', user: { id: 'u9' } },
            { seatKey: 'p-3', queuePath: 'Player', displayName: 'Player · 3', user: { id: 'u7' } },
          ],
        }),
      )
      render(<GroupPage />)

      await user.click(screen.getByRole('button', { name: 'Start game' }))

      expect(mutations.startTable).toHaveBeenCalledWith('table-1')
    })

    it('surfaces a refused request instead of leaving the button looking pressed', async () => {
      const user = userEvent.setup()
      mutations.startTableBackfill.mockRejectedValue(new Error('table backfill already active'))
      currentRoom = makeRoom(withQueue())
      render(<GroupPage />)

      await user.click(screen.getByRole('button', { name: 'Find us a match' }))

      expect(await screen.findByText('table backfill already active')).toBeInTheDocument()
    })
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

/**
 * Starting the game does not end the group — it is the point of it. The table leaves
 * `room.tables` the instant it starts (`Room.tables` only returns forming tables), so
 * every player except the one who pressed Start used to watch the page they were
 * waiting on turn into "This group has ended."
 */
describe('GroupPage when the game starts', () => {
  function makeIntent(overrides = {}) {
    return {
      activeIntent: null,
      activeTableSeat: null,
      loading: false,
      busy: false,
      leaveError: null,
      handleLeave: vi.fn(),
      ...overrides,
    }
  }

  const startedSeat = {
    tableId: 'table-1',
    status: 'started',
    gameName: 'Word Hunt',
    modeName: 'Arena',
    seatDisplayName: 'Player · 2',
    joinUrl: 'https://game.example/play?token=friend',
  }

  // Nobody in a friend room is surprised the game started — they were watching the
  // button. Waiting out the queue's countdown only strands them behind the host.
  it('launches the group without the queue\'s countdown', () => {
    currentRoom = { ...makeRoom(makeTable()), tables: [] }

    render(<GroupPage intent={makeIntent({ activeTableSeat: startedSeat })} />)

    expect(navigateToLaunchUrl).toHaveBeenCalledWith(startedSeat.joinUrl)
    expect(screen.queryByText(/Entering in/)).toBeNull()
  })

  it('takes a player who did not press Start into the match their seat says began', () => {
    currentRoom = { ...makeRoom(makeTable()), tables: [] }

    render(<GroupPage intent={makeIntent({ activeTableSeat: startedSeat })} />)

    fireEvent.click(screen.getByRole('button', { name: 'Launch Now' }))
    expect(navigateToLaunchUrl).toHaveBeenCalledWith(startedSeat.joinUrl)
    expect(screen.queryByText('This group has ended.')).not.toBeInTheDocument()
  })

  // The per-user seat event and the room-wide table event race, so the seat may say
  // "started" while the table is still in the room snapshot. The launch moment wins.
  it('launches even when the started table has not left the room snapshot yet', () => {
    currentRoom = makeRoom(makeTable())

    render(<GroupPage intent={makeIntent({ activeTableSeat: startedSeat })} />)

    expect(screen.getByRole('button', { name: 'Launch Now' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /claim/i })).not.toBeInTheDocument()
  })

  // The table disappearing is not evidence the group ended: the seat has to answer first.
  it('does not declare the group over while the seat is still being resolved', () => {
    currentRoom = { ...makeRoom(makeTable()), tables: [] }

    render(<GroupPage intent={makeIntent({ loading: true })} />)

    expect(screen.queryByText('This group has ended.')).not.toBeInTheDocument()
  })

  it('still says the group ended once the seat has answered and there is none', () => {
    currentRoom = { ...makeRoom(makeTable()), tables: [] }

    render(<GroupPage intent={makeIntent()} />)

    expect(screen.getByText('This group has ended.')).toBeInTheDocument()
  })
})
