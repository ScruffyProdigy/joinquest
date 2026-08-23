import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import TableCard from './TableCard'

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => ({
    user: {
      id: 'user-1',
      displayName: 'Pat',
      avatarKey: 'campfire',
      avatarUrl: '/avatars/campfire.png',
    },
  }),
}))

describe('TableCard', () => {
  it('shows the signed-in user avatar on role-layout seats', () => {
    const table = {
      id: 'table-1',
      game: { name: 'Codenames' },
      mode: {
        displayName: 'Party',
        queuePaths: [
          { queuePath: 'ClueGiver', minPlayers: 2, maxPlayers: 3 },
          { queuePath: 'Guesser', minPlayers: 2, maxPlayers: 6 },
        ],
      },
      seats: [{ seatKey: 'ClueGiver-Red', user: { id: 'user-1', displayName: 'Pat' } }],
      seatSlots: [
        {
          seatKey: 'ClueGiver-Red',
          queuePath: 'ClueGiver',
          displayName: 'Clue Giver · Red',
          user: { id: 'user-1', displayName: 'Pat' },
        },
        {
          seatKey: 'Guesser-1',
          queuePath: 'Guesser',
          displayName: 'Guesser · 1',
          user: null,
        },
      ],
      lookForGroupOptions: [],
    }

    const { container } = render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    const avatar = container.querySelector('[data-slot="avatar-image"][src="/avatars/campfire.png"]')
    expect(avatar).toBeTruthy()
    expect(screen.getByText('You')).toBeInTheDocument()
    expect(screen.getByText('Red')).toBeInTheDocument()
  })

  it('uses a single Sit button for pooled clue giver seats', () => {
    const table = {
      id: 'table-1',
      game: { name: 'WordHunt' },
      mode: {
        displayName: 'Party',
        queuePaths: [
          { queuePath: 'ClueGiver', minPlayers: 2, maxPlayers: 3 },
          { queuePath: 'Guesser', minPlayers: 2, maxPlayers: 6 },
        ],
      },
      seats: [],
      seatSlots: [
        { seatKey: 'ClueGiver-Red', queuePath: 'ClueGiver', displayName: 'Clue Giver · Red', user: null },
        { seatKey: 'ClueGiver-Blue', queuePath: 'ClueGiver', displayName: 'Clue Giver · Blue', user: null },
        { seatKey: 'Guesser-1', queuePath: 'Guesser', displayName: 'Guesser · 1', user: null },
      ],
      lookForGroupOptions: [],
    }

    render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    expect(screen.getByText('0/3 seated · need 2 to start')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Sit' })).toHaveLength(2)
  })

  it('uses a single Sit button for pooled guesser seats', () => {
    const table = {
      id: 'table-1',
      game: { name: 'WordHunt' },
      mode: {
        displayName: 'Party',
        queuePaths: [
          { queuePath: 'ClueGiver', minPlayers: 2, maxPlayers: 3 },
          { queuePath: 'Guesser', minPlayers: 2, maxPlayers: 6 },
        ],
      },
      seats: [],
      seatSlots: [
        { seatKey: 'ClueGiver-Red', queuePath: 'ClueGiver', displayName: 'Clue Giver · Red', user: null },
        { seatKey: 'Guesser-1', queuePath: 'Guesser', displayName: 'Guesser · 1', user: null },
        { seatKey: 'Guesser-2', queuePath: 'Guesser', displayName: 'Guesser · 2', user: null },
      ],
      lookForGroupOptions: [],
    }

    render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    expect(screen.getByText('0/6 seated · need 2 to start')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Sit' })).toHaveLength(2)
  })

  it('shows Look for group alongside Start now when the table can start but has open seats', () => {
    const table = {
      id: 'table-1',
      game: { name: 'WordHunt' },
      mode: {
        displayName: 'Party',
        queuePaths: [
          { queuePath: 'ClueGiver', minPlayers: 2, maxPlayers: 3 },
          { queuePath: 'Guesser', minPlayers: 2, maxPlayers: 6 },
        ],
      },
      king: { id: 'user-1', displayName: 'Pat' },
      canStart: true,
      seats: [
        { seatKey: 'ClueGiver-Red', user: { id: 'user-1', displayName: 'Pat' } },
        { seatKey: 'ClueGiver-Blue', user: { id: 'user-2', displayName: 'Sam' } },
        { seatKey: 'Guesser-1', user: { id: 'user-3', displayName: 'Alex' } },
        { seatKey: 'Guesser-2', user: { id: 'user-4', displayName: 'Jordan' } },
      ],
      seatSlots: [
        { seatKey: 'ClueGiver-Red', queuePath: 'ClueGiver', displayName: 'Clue Giver · Red', user: { id: 'user-1', displayName: 'Pat' } },
        { seatKey: 'ClueGiver-Blue', queuePath: 'ClueGiver', displayName: 'Clue Giver · Blue', user: { id: 'user-2', displayName: 'Sam' } },
        { seatKey: 'Guesser-1', queuePath: 'Guesser', displayName: 'Guesser · 1', user: { id: 'user-3', displayName: 'Alex' } },
        { seatKey: 'Guesser-2', queuePath: 'Guesser', displayName: 'Guesser · 2', user: { id: 'user-4', displayName: 'Jordan' } },
      ],
      lookForGroupOptions: [{ queueId: 'queue-1', queueName: 'Default', visible: true, enabled: false }],
    }

    render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    expect(screen.getByRole('button', { name: 'Start now' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Look for group (Default)' })).toBeInTheDocument()
  })

  it('hides Look for group from non-kings when options are visible', () => {
    const table = {
      id: 'table-1',
      game: { name: 'WordHunt' },
      mode: {
        displayName: 'Party',
        queuePaths: [
          { queuePath: 'ClueGiver', minPlayers: 2, maxPlayers: 3 },
          { queuePath: 'Guesser', minPlayers: 2, maxPlayers: 6 },
        ],
      },
      king: { id: 'user-2', displayName: 'Sam' },
      canStart: true,
      seats: [
        { seatKey: 'ClueGiver-Red', user: { id: 'user-1', displayName: 'Pat' } },
        { seatKey: 'ClueGiver-Blue', user: { id: 'user-2', displayName: 'Sam' } },
        { seatKey: 'Guesser-1', user: { id: 'user-3', displayName: 'Alex' } },
        { seatKey: 'Guesser-2', user: { id: 'user-4', displayName: 'Jordan' } },
      ],
      seatSlots: [
        { seatKey: 'ClueGiver-Red', queuePath: 'ClueGiver', displayName: 'Clue Giver · Red', user: { id: 'user-1', displayName: 'Pat' } },
        { seatKey: 'ClueGiver-Blue', queuePath: 'ClueGiver', displayName: 'Clue Giver · Blue', user: { id: 'user-2', displayName: 'Sam' } },
        { seatKey: 'Guesser-1', queuePath: 'Guesser', displayName: 'Guesser · 1', user: { id: 'user-3', displayName: 'Alex' } },
        { seatKey: 'Guesser-2', queuePath: 'Guesser', displayName: 'Guesser · 2', user: { id: 'user-4', displayName: 'Jordan' } },
      ],
      lookForGroupOptions: [{ queueId: 'queue-1', queueName: 'Default', visible: true, enabled: false }],
    }

    render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    expect(screen.queryByRole('button', { name: 'Look for group (Default)' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Start now' })).not.toBeInTheDocument()
  })

  it('highlights the king with a gold avatar ring', () => {
    const table = {
      id: 'table-1',
      game: { name: 'WordHunt' },
      mode: {
        displayName: 'Party',
        queuePaths: [
          { queuePath: 'ClueGiver', minPlayers: 2, maxPlayers: 3 },
          { queuePath: 'Guesser', minPlayers: 2, maxPlayers: 6 },
        ],
      },
      king: { id: 'user-2', displayName: 'Sam' },
      seats: [
        { seatKey: 'ClueGiver-Red', user: { id: 'user-1', displayName: 'Pat' } },
        { seatKey: 'ClueGiver-Blue', user: { id: 'user-2', displayName: 'Sam' } },
      ],
      seatSlots: [
        { seatKey: 'ClueGiver-Red', queuePath: 'ClueGiver', displayName: 'Clue Giver · Red', user: { id: 'user-1', displayName: 'Pat' } },
        { seatKey: 'ClueGiver-Blue', queuePath: 'ClueGiver', displayName: 'Clue Giver · Blue', user: { id: 'user-2', displayName: 'Sam' } },
      ],
      lookForGroupOptions: [],
    }

    const { container } = render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    expect(container.querySelectorAll('[data-slot="avatar"][data-ring="king"]')).toHaveLength(1)
    expect(screen.getByText(/King: Sam/)).toBeInTheDocument()
  })

  it('uses a single Sit button for fifo duel seats', () => {
    const table = {
      id: 'table-1',
      game: { name: 'Rock Paper Scissors Lizard Robot' },
      mode: { displayName: '1v1 Duel (best 3 of 5)' },
      seats: [],
      seatSlots: [
        { seatKey: '1', displayName: '1', user: null },
        { seatKey: '2', displayName: '2', user: null },
      ],
      lookForGroupOptions: [],
    }

    render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    expect(screen.getByRole('heading', { name: 'Players' })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Sit' })).toHaveLength(1)
  })

  it('shows catalog icon, blurb, and tags to help unfamiliar players', () => {
    const table = {
      id: 'table-1',
      game: {
        name: 'Word Hunt',
        iconUrl: '/games/word-hunt-icon.png',
        shortDescription: 'Everyone competes on a shared word grid.',
        tags: ['party', 'competitive', 'words'],
      },
      mode: { displayName: 'Party' },
      seats: [],
      seatSlots: [],
      lookForGroupOptions: [],
    }

    const { container } = render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    expect(container.querySelector('img[src="/games/word-hunt-icon.png?v=1"]')).toBeInTheDocument()
    expect(screen.getByText('Everyone competes on a shared word grid.')).toBeInTheDocument()
    expect(screen.getByText('competitive')).toBeInTheDocument()
  })

  it('shows forming gaps when the table still needs lobby players', () => {
    const table = {
      id: 'table-1',
      game: { name: 'Word Hunt' },
      mode: { displayName: 'Party' },
      seats: [{ seatKey: 'ClueGiver-Red', user: { id: 'user-1', displayName: 'Pat' } }],
      seatSlots: [
        { seatKey: 'ClueGiver-Red', queuePath: 'ClueGiver', displayName: 'Clue Giver · Red', user: { id: 'user-1', displayName: 'Pat' } },
        { seatKey: 'Guesser-1', queuePath: 'Guesser', displayName: 'Guesser · 1', user: null },
      ],
      lookForGroupOptions: [{ queueId: 'q1', queueName: 'Default', visible: true, enabled: false }],
      backfillActive: false,
      formingGaps: [
        { queuePath: 'ClueGiver', displayName: 'Clue Giver', assigned: 1, needed: 1 },
        { queuePath: 'Guesser', displayName: 'Guesser', assigned: 0, needed: 4 },
      ],
    }

    render(
      <TableCard table={table} busy={false} onSit={() => {}} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    expect(screen.getByText('Need 1 Clue Giver, 4 Guesser from the lobby')).toBeInTheDocument()
  })

  it('opens a seat-picker sheet on Sit and confirms the seat on confirm', async () => {
    const user = userEvent.setup()
    const onSit = vi.fn()
    const table = {
      id: 'table-1',
      game: { name: 'Rock Paper Scissors Lizard Robot' },
      mode: { displayName: '1v1 Duel (best 3 of 5)' },
      seats: [],
      seatSlots: [
        { seatKey: '1', displayName: '1', user: null },
        { seatKey: '2', displayName: '2', user: { id: 'user-2', displayName: 'Sam' } },
      ],
      lookForGroupOptions: [],
    }

    render(
      <TableCard table={table} busy={false} onSit={onSit} onLeave={() => {}} onStart={() => {}} onDiscard={() => {}} />,
    )

    await user.click(screen.getByRole('button', { name: 'Sit' }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(sheet.getByRole('heading', { name: 'Choose your seat', level: 2 })).toBeInTheDocument()
    expect(sheet.getByText('Players')).toBeInTheDocument()
    expect(sheet.getByText('Sam')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Confirm seat' }))
    expect(onSit).toHaveBeenCalledWith('1')
  })
})
