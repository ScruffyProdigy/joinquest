import { describe, expect, it, vi, beforeEach } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import { ActiveRoomProvider, useActiveRoom } from './ActiveRoomProvider'
import { fetchMyRoom, subscribeToRoom } from '../../lib/rooms'
import { fetchMyTableSeat } from '../../lib/tables'

const authState = { user: null, loading: false }

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => authState,
}))

vi.mock('../../lib/rooms', () => ({
  fetchMyRoom: vi.fn(),
  fetchRoom: vi.fn(),
  joinRoom: vi.fn(),
  leaveRoom: vi.fn(),
  subscribeToRoom: vi.fn(),
}))

vi.mock('../../lib/tables', async (importOriginal) => ({
  ...(await importOriginal()),
  fetchMyTableSeat: vi.fn(),
}))

vi.mock('../../lib/queue', () => ({
  prefetchSubscriptionAuth: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('../../lib/usePathname', () => ({
  navigateTo: vi.fn(),
}))

vi.mock('../../lib/tabVisibility', () => ({
  onTabVisible: vi.fn(() => () => {}),
}))

function room(inviteCode) {
  return { id: `room-${inviteCode}`, inviteCode, members: [], tables: [], messages: [] }
}

/** Holds on to the `refresh` from the very first render, the way a click handler does. */
let capturedRefresh = null
/** Every room code the provider has published, so a wipe cannot hide behind a reload. */
let renderedCodes = []

function Probe() {
  const { room: activeRoom, refresh } = useActiveRoom()
  if (!capturedRefresh) {
    capturedRefresh = refresh
  }
  const code = activeRoom?.inviteCode ?? 'none'
  renderedCodes.push(code)
  return <div data-testid="room">{code}</div>
}

function renderProvider() {
  return render(
    <ActiveRoomProvider>
      <Probe />
    </ActiveRoomProvider>,
  )
}

function currentRoomCode() {
  return screen.getByTestId('room').textContent
}

/** Stand in for signing in / a guest session being created mid-flight. */
async function signIn(rerender) {
  authState.user = { id: 'user-1', displayName: 'Pat' }
  await act(async () => {
    rerender(
      <ActiveRoomProvider>
        <Probe />
      </ActiveRoomProvider>,
    )
  })
}

describe('ActiveRoomProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    capturedRefresh = null
    renderedCodes = []
    authState.user = null
    authState.loading = false
    fetchMyRoom.mockResolvedValue(null)
    fetchMyTableSeat.mockResolvedValue(null)
    subscribeToRoom.mockResolvedValue(() => {})
  })

  it('loads the room from a refresh captured before the session existed', async () => {
    const { rerender } = renderProvider()
    const staleRefresh = capturedRefresh
    expect(staleRefresh).toBeTypeOf('function')

    fetchMyRoom.mockResolvedValue(room('ABCD'))
    await signIn(rerender)
    await waitFor(() => expect(currentRoomCode()).toBe('ABCD'))

    // The room the visitor just created, seen by a caller holding the
    // session-less closure (JQ-84): it must load, not wipe. A later reload can
    // put the room back, so the wipe itself is what this asserts on.
    fetchMyRoom.mockResolvedValue(room('WXYZ'))
    const mark = renderedCodes.length
    await act(async () => {
      await staleRefresh()
    })

    expect(renderedCodes.slice(mark)).not.toContain('none')
    expect(currentRoomCode()).toBe('WXYZ')
  })

  it('clears the room when the session goes away', async () => {
    fetchMyRoom.mockResolvedValue(room('ABCD'))
    const { rerender } = renderProvider()
    await signIn(rerender)
    await waitFor(() => expect(currentRoomCode()).toBe('ABCD'))

    authState.user = null
    await act(async () => {
      rerender(
        <ActiveRoomProvider>
          <Probe />
        </ActiveRoomProvider>,
      )
    })

    await waitFor(() => expect(currentRoomCode()).toBe('none'))
  })

  it('does not load a room for a refresh called while signed out', async () => {
    renderProvider()

    await act(async () => {
      await capturedRefresh()
    })

    expect(fetchMyRoom).not.toHaveBeenCalled()
    expect(currentRoomCode()).toBe('none')
  })
})
