import { render, screen } from '@testing-library/react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import App from '../../App'
import { mockAuthenticatedSession } from '../../test/setup'

// Loading a real room starts the room WebSocket. jsdom has no server, and the client
// retries with backoff (lazy: false, shouldRetry: true), which starves the other test
// files in this worker. Nothing here is testing live updates, so stub the subscription.
vi.mock('../../lib/rooms', async (importOriginal) => ({
  ...(await importOriginal()),
  subscribeToRoom: async () => () => {},
}))

function goTo(pathname) {
  window.history.replaceState({}, '', pathname)
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, pathname, search: '', assign: vi.fn() },
  })
}

const roomWithOneTable = {
  id: 'room-1',
  inviteCode: 'ABC123',
  joinUrl: 'https://joinquest.cc/room/ABC123',
  host: { id: 'user-1', displayName: 'player' },
  members: [{ user: { id: 'user-1', displayName: 'player' }, disconnected: false }],
  messages: [],
  tables: [
    {
      id: 'table-1',
      createdAt: '2026-09-01T00:00:00Z',
      canStart: false,
      canDiscard: true,
      game: { id: 'game-1', slug: 'word-hunt', name: 'Word Hunt', accentColor: null },
      mode: { id: 'mode-1', displayName: 'Arena', queuePaths: [] },
      king: { id: 'user-1', displayName: 'player' },
      seats: [],
      seatSlots: [
        { seatKey: 'p-1', queuePath: 'Player', displayName: 'Player · 1', user: null },
        { seatKey: 'p-2', queuePath: 'Player', displayName: 'Player · 2', user: null },
      ],
      lookForGroupOptions: [],
      backfillActive: false,
      formingGaps: [{ queuePath: 'Player', displayName: 'Player', assigned: 0, needed: 2 }],
      regroupRoster: [],
    },
  ],
}

describe('the /group route', () => {
  beforeEach(() => {
    goTo('/')
  })

  it('shows the loaded group', async () => {
    goTo('/group')
    mockAuthenticatedSession(undefined, { myRoom: roomWithOneTable })

    render(<App />)

    // Assert on something only the loaded page renders: the empty state also
    // carries the "Your Group" heading, so that alone would pass vacuously.
    expect(await screen.findByRole('region', { name: /invite friends/i })).toBeInTheDocument()
    expect(screen.getByText('Still need: Player')).toBeInTheDocument()
  })
})
