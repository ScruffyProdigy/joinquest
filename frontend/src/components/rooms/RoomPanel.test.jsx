import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import RoomPanel from './RoomPanel'

const mockRoom = {
  id: 'room-1',
  inviteCode: 'ABCD',
  joinUrl: 'https://joinquest.cc/r/abcd',
  host: { id: 'user-1' },
  members: [{ user: { id: 'user-1', displayName: 'Pat' }, away: false }],
  tables: [],
}

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => ({ user: { id: 'user-1', displayName: 'Pat' }, loading: false }),
}))

vi.mock('./ActiveRoomProvider', () => ({
  useActiveRoom: () => ({
    room: mockRoom,
    messages: [],
    setMessages: vi.fn(),
    loading: false,
    error: '',
    setError: vi.fn(),
    handleLeave: vi.fn(),
    mergeTableUpdate: vi.fn(),
    refresh: vi.fn(),
    unreadCount: 0,
    markRead: vi.fn(),
  }),
}))

vi.mock('../games/useActiveTableSeat', () => ({
  useActiveTableSeat: () => ({ refresh: vi.fn() }),
}))

vi.mock('../games/useActiveIntent', () => ({
  useActiveIntent: () => ({ refresh: vi.fn() }),
}))

describe('RoomPanel', () => {
  it('shows the room invite code and member count', () => {
    render(<RoomPanel compact />)
    expect(screen.getByText('Room ABCD')).toBeInTheDocument()
    expect(screen.getByText('1 member')).toBeInTheDocument()
  })

  it('shows the leave room button', () => {
    render(<RoomPanel compact />)
    expect(screen.getByRole('button', { name: /Leave room/ })).toBeInTheDocument()
  })

  // JQ-265. The member stays listed and still counts — the roster's job here is to stop
  // claiming they are watching, not to start pretending they left. A regression that
  // filtered away members out would pass a "shows away" assertion on its own, so the
  // count and the row are asserted together.
  it('dims an away member without dropping them from the roster', () => {
    const original = mockRoom.members
    mockRoom.members = [
      { user: { id: 'user-1', displayName: 'Pat' }, away: false },
      { user: { id: 'user-2', displayName: 'Sam' }, away: true },
    ]
    try {
      const { container } = render(<RoomPanel compact />)
      expect(screen.getByText('2 members')).toBeInTheDocument()
      expect(screen.getByText(/Sam/)).toBeInTheDocument()
      expect(screen.getByText(/Sam — away/)).toBeInTheDocument()
      const dimmed = container.querySelectorAll('[data-slot="avatar"][data-away="true"]')
      expect(dimmed).toHaveLength(1)
      expect(dimmed[0].getAttribute('title')).toBe('Sam (away)')
    } finally {
      mockRoom.members = original
    }
  })

  it('leaves a present member undimmed', () => {
    const { container } = render(<RoomPanel compact />)
    expect(container.querySelector('[data-slot="avatar"][data-away="true"]')).toBeNull()
    expect(screen.queryByText(/away/)).toBeNull()
  })
})
