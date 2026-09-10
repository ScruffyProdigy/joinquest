import { render, screen } from '@testing-library/react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import App from '../../App'
import { mockAuthenticatedSession, mockDemoGames } from '../../test/setup'

// Loading a real room starts the room WebSocket. jsdom has no server, and the client
// retries with backoff (lazy: false, shouldRetry: true), which starves the other test
// files in this worker. Nothing here is testing live updates, so stub the subscription.
vi.mock('../../lib/rooms', async (importOriginal) => ({
  ...(await importOriginal()),
  subscribeToRoom: async () => () => {},
}))

const room = {
  id: 'room-1',
  inviteCode: 'ABC123',
  joinUrl: 'https://joinquest.cc/room/ABC123',
  host: { id: 'user-1', displayName: 'player' },
  members: [{ user: { id: 'user-1', displayName: 'player' }, away: false }],
  messages: [],
  tables: [],
}

function goTo(pathname) {
  window.history.replaceState({}, '', pathname)
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, pathname, search: '', assign: vi.fn() },
  })
}

describe('the /room/:inviteCode route', () => {
  beforeEach(() => {
    goTo('/')
  })

  // JQ-206 took the dock and the room surfaces out of the UI. Invite links are
  // already in the wild, so the route itself has to keep resolving — it must not
  // fall through to the not-found shell just because nothing docks to it now.
  it('still resolves for a signed-in visitor instead of falling through to not found', async () => {
    goTo('/room/ABC123')
    mockAuthenticatedSession(undefined, { myRoom: room, games: mockDemoGames })

    render(<App />)

    expect(await screen.findByRole('heading', { level: 1, name: 'Solo or squad, just join.' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Page not found' })).not.toBeInTheDocument()
  })

  it('renders no dock, room sheet or room chat panel', async () => {
    goTo('/room/ABC123')
    mockAuthenticatedSession(undefined, { myRoom: room, games: mockDemoGames })

    render(<App />)

    await screen.findByRole('heading', { level: 1, name: 'Solo or squad, just join.' })
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).not.toBeInTheDocument()
    expect(screen.queryByRole('complementary', { name: 'Room chat' })).not.toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: 'Room chat' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /unread messages/ })).not.toBeInTheDocument()
  })
})
