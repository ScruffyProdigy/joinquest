import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import RoomPanel from './RoomPanel'

const mockRoom = {
  id: 'room-1',
  inviteCode: 'ABCD',
  joinUrl: 'https://joinquest.cc/r/abcd',
  host: { id: 'user-1' },
  members: [{ id: 'user-1', displayName: 'Pat' }],
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
})
