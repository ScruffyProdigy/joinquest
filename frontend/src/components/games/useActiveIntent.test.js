import { renderHook, act, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useActiveIntent } from './useActiveIntent'
import * as intent from '../../lib/intent'
import { LEAVE_GAME_FAILED, LEAVE_GAME_NOT_FOUND, REJOIN_FAILED, REJOIN_OVER } from '../../lib/playerCopy'

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
    rejoinActiveMatch: vi.fn(),
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

describe('useActiveIntent handleRejoin', () => {
  const originalLocation = window.location

  beforeEach(() => {
    vi.mocked(intent.fetchMyActiveIntent).mockReset()
    vi.mocked(intent.fetchMyActiveIntent).mockResolvedValue(playingIntent)
    vi.mocked(intent.rejoinActiveMatch).mockReset()
    // jsdom refuses a real navigation, so the assign target is what we assert on.
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...originalLocation, assign: vi.fn() },
    })
  })

  afterEach(() => {
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
  })

  it('sends the player to the URL minted at click time, not a cached one', async () => {
    vi.mocked(intent.rejoinActiveMatch).mockResolvedValue('https://example.test/play?token=fresh')
    const result = await renderPlaying()

    await act(async () => {
      await result.current.handleRejoin()
    })

    expect(intent.rejoinActiveMatch).toHaveBeenCalled()
    expect(window.location.assign).toHaveBeenCalledWith('https://example.test/play?token=fresh')
    expect(result.current.rejoinError).toBeNull()
    expect(result.current.rejoining).toBe(false)
  })

  it('reports a failure and stays put when no URL comes back', async () => {
    vi.mocked(intent.rejoinActiveMatch).mockResolvedValue(null)
    const result = await renderPlaying()

    await act(async () => {
      await result.current.handleRejoin()
    })

    expect(window.location.assign).not.toHaveBeenCalled()
    expect(result.current.rejoinError).toBe(REJOIN_FAILED)
    expect(result.current.rejoining).toBe(false)
  })

  // The server refuses once the match is over, which means the banner we acted on
  // was stale — so the refusal has to clear it rather than leave a dead action up.
  it('clears a stale banner when the server refuses the rejoin', async () => {
    vi.mocked(intent.rejoinActiveMatch).mockRejectedValue(new Error('no live match to rejoin'))
    const result = await renderPlaying()

    vi.mocked(intent.fetchMyActiveIntent).mockResolvedValue(null)
    await act(async () => {
      await result.current.handleRejoin()
    })

    expect(window.location.assign).not.toHaveBeenCalled()
    expect(result.current.rejoinError).toBe(REJOIN_OVER)
    await waitFor(() => {
      expect(result.current.activeIntent).toBeNull()
    })
  })
})
