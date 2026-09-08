import { render, act, waitFor } from '@testing-library/react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useLeaveQueueOnExit } from './useLeaveQueueOnExit'
import * as queue from '../../lib/queue'
import { navigateTo } from '../../lib/usePathname'

vi.mock('../../lib/queue', () => ({
  leaveQueue: vi.fn(() => Promise.resolve(true)),
  leaveQueueOnExit: vi.fn(),
}))

function Probe({ activeIntent }) {
  useLeaveQueueOnExit(activeIntent)
  return null
}

const waitingIntent = { queueId: 'q1', status: 'WAITING' }

describe('useLeaveQueueOnExit', () => {
  beforeEach(() => {
    vi.mocked(queue.leaveQueue).mockClear()
    vi.mocked(queue.leaveQueueOnExit).mockClear()
    window.history.replaceState(null, '', '/waiting')
  })

  it('gives up the queue when the player routes away while still queued', () => {
    render(<Probe activeIntent={waitingIntent} />)

    act(() => navigateTo('/games/word-hunt'))

    expect(queue.leaveQueue).toHaveBeenCalledWith('q1')
  })

  it('does not leave when the route change is the waiting page letting go itself', () => {
    // Stop looking confirmed, or a match formed: the intent is already gone by the
    // time the page routes out, so there is nothing left to leave.
    const { rerender } = render(<Probe activeIntent={waitingIntent} />)
    rerender(<Probe activeIntent={null} />)

    act(() => navigateTo('/games/word-hunt'))

    expect(queue.leaveQueue).not.toHaveBeenCalled()
  })

  it('does not leave on a route change that stays on the waiting page', () => {
    render(<Probe activeIntent={waitingIntent} />)

    act(() => navigateTo('/waiting'))

    expect(queue.leaveQueue).not.toHaveBeenCalled()
  })

  it('leaves a matched intent alone', () => {
    render(<Probe activeIntent={{ queueId: 'q1', status: 'MATCHED' }} />)

    act(() => navigateTo('/games/word-hunt'))

    expect(queue.leaveQueue).not.toHaveBeenCalled()
  })

  it('uses the unload-safe leave when the document is torn down', () => {
    render(<Probe activeIntent={waitingIntent} />)

    act(() => {
      window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: false }))
    })

    expect(queue.leaveQueueOnExit).toHaveBeenCalledWith('q1')
    expect(queue.leaveQueue).not.toHaveBeenCalled()
  })

  it('stays in the queue when the page is only frozen into the back/forward cache', async () => {
    render(<Probe activeIntent={waitingIntent} />)

    act(() => {
      window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }))
    })

    expect(queue.leaveQueueOnExit).not.toHaveBeenCalled()
  })

  it('puts the player back on the waiting page when the leave fails', async () => {
    vi.mocked(queue.leaveQueue).mockRejectedValueOnce(new Error('offline'))
    render(<Probe activeIntent={waitingIntent} />)

    act(() => navigateTo('/games/word-hunt'))
    await waitFor(() => expect(window.location.pathname).toBe('/waiting'))
  })

  it('survives a StrictMode remount without dropping the player', () => {
    // The dev double-mount unmounts and remounts with no route change. Keying off
    // the route rather than the unmount is what keeps this from leaving the queue.
    const { unmount } = render(<Probe activeIntent={waitingIntent} />)
    unmount()
    render(<Probe activeIntent={waitingIntent} />)

    expect(queue.leaveQueue).not.toHaveBeenCalled()
    expect(queue.leaveQueueOnExit).not.toHaveBeenCalled()
  })

  it('stops listening once the page is gone', () => {
    const { unmount } = render(<Probe activeIntent={waitingIntent} />)
    unmount()

    act(() => navigateTo('/games/word-hunt'))

    expect(queue.leaveQueue).not.toHaveBeenCalled()
  })
})
