import { renderHook, act, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useGameQueue } from './useGameQueue'
import * as queue from '../../lib/queue'

vi.mock('../../lib/queue', () => ({
  joinQueue: vi.fn(),
  fetchMyQueueStatus: vi.fn(),
  leaveQueue: vi.fn(),
  prefetchSubscriptionAuth: vi.fn().mockResolvedValue('Bearer test-token'),
  subscribeToQueue: vi.fn(),
}))

const queueId = 'queue-1'

describe('useGameQueue', () => {
  beforeEach(() => {
    vi.mocked(queue.joinQueue).mockReset()
    vi.mocked(queue.fetchMyQueueStatus).mockReset()
    vi.mocked(queue.leaveQueue).mockReset()
    vi.mocked(queue.prefetchSubscriptionAuth).mockReset()
    vi.mocked(queue.prefetchSubscriptionAuth).mockResolvedValue('Bearer test-token')
    vi.mocked(queue.subscribeToQueue).mockReset()
    vi.mocked(queue.subscribeToQueue).mockResolvedValue(() => {})
  })

  it('does not sync from server when skipSubscription is true', async () => {
    vi.mocked(queue.fetchMyQueueStatus).mockResolvedValue({ queued: false })

    const { result } = renderHook(() => useGameQueue(queueId, { skipSubscription: true }))

    await waitFor(() => {
      expect(queue.fetchMyQueueStatus).not.toHaveBeenCalled()
      expect(result.current.queueState).toBe('idle')
    })
  })

  it('syncs waiting state from server on mount (cross-tab)', async () => {
    vi.mocked(queue.fetchMyQueueStatus).mockResolvedValue({
      queued: true,
      queuedCount: 2,
    })

    const { result } = renderHook(() => useGameQueue(queueId))

    await waitFor(() => {
      expect(result.current.queueState).toBe('waiting')
      expect(result.current.queuedCount).toBe(2)
      expect(queue.subscribeToQueue).toHaveBeenCalledWith(queueId, expect.any(Object))
    })
  })

  it('enters waiting state when join is queued', async () => {
    vi.mocked(queue.joinQueue).mockResolvedValue({ queued: true, queuedCount: 1 })
    vi.mocked(queue.fetchMyQueueStatus).mockResolvedValue({ queued: false })

    const { result } = renderHook(() => useGameQueue(queueId))

    await act(async () => {
      await result.current.handleJoin('DPS')
    })

    await waitFor(() => {
      expect(result.current.queueState).toBe('waiting')
      expect(result.current.selectedQueuePath).toBe('DPS')
      // A mode with no pre-queue picker passes no options through.
      expect(queue.joinQueue).toHaveBeenCalledWith(queueId, 'DPS', undefined)
    })
  })

  it('shows launch when join completes the match (second player)', async () => {
    vi.mocked(queue.fetchMyQueueStatus).mockResolvedValue({ queued: false })
    vi.mocked(queue.joinQueue).mockResolvedValue({
      queued: false,
      joinUrl: 'http://localhost:5174/?match=s2&token=eyJ.test',
    })

    const { result } = renderHook(() => useGameQueue(queueId))

    await act(async () => {
      await result.current.handleJoin()
    })

    expect(result.current.queueState).toBe('matched')
    expect(result.current.joinUrl).toContain('match=s2')
  })

  it('shows launch when subscription delivers a match', async () => {
    vi.mocked(queue.fetchMyQueueStatus).mockResolvedValue({ queued: false })
    vi.mocked(queue.joinQueue).mockResolvedValue({ queued: true, queuedCount: 1 })
    let onUpdate
    vi.mocked(queue.subscribeToQueue).mockImplementation(async (_id, handlers) => {
      onUpdate = handlers.onUpdate
      return () => {}
    })

    const { result } = renderHook(() => useGameQueue(queueId))

    await act(async () => {
      await result.current.handleJoin()
    })

    await waitFor(() => expect(queue.subscribeToQueue).toHaveBeenCalled())

    act(() => {
      onUpdate({
        status: 'MATCHED',
        joinUrl: 'http://localhost:5174/?match=session-2&token=eyJ.test',
        queuedCount: 0,
      })
    })

    await waitFor(() => {
      expect(result.current.queueState).toBe('matched')
      expect(result.current.joinUrl).toBe('http://localhost:5174/?match=session-2&token=eyJ.test')
    })
  })

  it('reports subscription errors', async () => {
    vi.mocked(queue.fetchMyQueueStatus).mockResolvedValue({ queued: false })
    vi.mocked(queue.joinQueue).mockResolvedValue({ queued: true, queuedCount: 1 })
    let onError
    vi.mocked(queue.subscribeToQueue).mockImplementation(async (_id, handlers) => {
      onError = handlers.onError
      return () => {}
    })

    const { result } = renderHook(() => useGameQueue(queueId))

    await act(async () => {
      await result.current.handleJoin()
    })

    await waitFor(() => expect(onError).toBeDefined())

    act(() => {
      onError('authentication required')
    })

    expect(result.current.error).toMatch(/authentication required/i)
  })
})
