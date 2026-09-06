import { renderHook, act, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useActiveIntent } from './useActiveIntent'
import * as intent from '../../lib/intent'
import { LEAVE_GAME_FAILED, LEAVE_GAME_NOT_FOUND } from '../../lib/playerCopy'

// One stable auth object for the whole file: the hooks key their callbacks on
// `user` identity, so a fresh object per render re-runs every effect forever.
const AUTH = { user: { id: 'user-1' }, loading: false }

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => AUTH,
}))

vi.mock('../../lib/intent', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchMyActiveIntent: vi.fn(),
    leaveActiveGame: vi.fn(),
  }
})

vi.mock('../../lib/queue', () => ({
  fetchMyQueueStatus: vi.fn().mockResolvedValue(null),
  leaveQueue: vi.fn().mockResolvedValue(undefined),
  onQueueWsConnectionChange: vi.fn(() => () => {}),
  prefetchSubscriptionAuth: vi.fn().mockResolvedValue('Bearer test-token'),
  subscribeToQueue: vi.fn().mockResolvedValue(() => {}),
}))

vi.mock('../../lib/tables', () => ({
  TABLE_UPDATED_EVENT: 'table-updated',
  fetchMyTableSeat: vi.fn().mockResolvedValue(null),
  leaveTable: vi.fn().mockResolvedValue(undefined),
  subscribeToMyTableSeat: vi.fn().mockResolvedValue(() => {}),
}))

// A join URL is part of the fixture on purpose: without one the hook starts a
// polling interval that keeps re-rendering while the test is asserting.
const playingIntent = {
  queueId: 'queue-1',
  gameId: 'game-1',
  gameName: 'Demo Game',
  status: 'MATCHED',
  joinUrl: 'https://example.test/play',
}

async function renderPlaying() {
  const { result } = renderHook(() => useActiveIntent())
  await waitFor(() => {
    expect(result.current.activeIntent?.status).toBe('MATCHED')
  })
  return result
}

describe('useActiveIntent handleLeave', () => {
  beforeEach(() => {
    vi.mocked(intent.fetchMyActiveIntent).mockReset()
    vi.mocked(intent.fetchMyActiveIntent).mockResolvedValue(playingIntent)
    vi.mocked(intent.leaveActiveGame).mockReset()
  })

  it('clears the banner and reports no error when the leave succeeds', async () => {
    vi.mocked(intent.leaveActiveGame).mockResolvedValue(true)
    const result = await renderPlaying()

    vi.mocked(intent.fetchMyActiveIntent).mockResolvedValue(null)
    await act(async () => {
      await result.current.handleLeave()
    })

    expect(intent.leaveActiveGame).toHaveBeenCalled()
    expect(result.current.leaveError).toBeNull()
    expect(result.current.busy).toBe(false)
  })

  it('surfaces a message when the server had nothing to leave', async () => {
    vi.mocked(intent.leaveActiveGame).mockResolvedValue(false)
    const result = await renderPlaying()

    await act(async () => {
      await result.current.handleLeave()
    })

    expect(result.current.leaveError).toBe(LEAVE_GAME_NOT_FOUND)
    expect(result.current.busy).toBe(false)
  })

  it('surfaces a message when the leave request throws', async () => {
    vi.mocked(intent.leaveActiveGame).mockRejectedValue(new Error('network down'))
    const result = await renderPlaying()

    await act(async () => {
      await result.current.handleLeave()
    })

    expect(result.current.leaveError).toBe(LEAVE_GAME_FAILED)
    expect(result.current.busy).toBe(false)
  })

  it('clears a previous error when the player tries again', async () => {
    vi.mocked(intent.leaveActiveGame).mockRejectedValue(new Error('network down'))
    const result = await renderPlaying()

    await act(async () => {
      await result.current.handleLeave()
    })
    expect(result.current.leaveError).toBe(LEAVE_GAME_FAILED)

    vi.mocked(intent.leaveActiveGame).mockResolvedValue(true)
    vi.mocked(intent.fetchMyActiveIntent).mockResolvedValue(null)
    await act(async () => {
      await result.current.handleLeave()
    })

    expect(result.current.leaveError).toBeNull()
  })
})
