import { render, act, waitFor } from '@testing-library/react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useLeaveQueueOnExit } from './useLeaveQueueOnExit'
import * as queue from '../../lib/queue'
import { navigateTo } from '../../lib/usePathname'
import { navigateOutOfWaiting, rememberWaitingReturnPath } from '../../lib/waiting'

vi.mock('../../lib/queue', () => ({
  leaveQueue: vi.fn(() => Promise.resolve(true)),
  leaveQueueOnExit: vi.fn(),
}))

function Probe({ activeIntent, onBackGesture }) {
  useLeaveQueueOnExit(activeIntent, onBackGesture)
  return null
}

/** A real browser Back — the iOS edge swipe raises exactly this. jsdom traverses
 * history for real, so nothing here has to pretend to be a pop. */
async function pressBack() {
  await act(async () => {
    const popped = new Promise((resolve) => {
      window.addEventListener('popstate', resolve, { once: true })
    })
    window.history.back()
    await popped
  })
}

/** Arrive on the waiting page the way a player does: from somewhere else. */
function arriveOnWaiting() {
  window.history.replaceState(null, '', '/games/word-hunt')
  rememberWaitingReturnPath()
  window.history.pushState(null, '', '/waiting')
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

  it('lets a resolved leave route away, including the one the server already handled', async () => {
    // Named for what it can actually pin. A server-side eviction (JQ-216) reaches
    // this hook as a leaveQueue that resolves normally — LeaveModeQueue swallows the
    // missing-row case and still returns true — so there is no client-side input
    // that distinguishes it from an ordinary successful leave, and no test here can
    // fail specifically for the eviction. What it does pin is that the recovery
    // above is failure-only: a resolved leave must not bounce the player back to the
    // waiting page.
    render(<Probe activeIntent={waitingIntent} />)

    act(() => navigateTo('/games/word-hunt'))
    await act(async () => {
      await queue.leaveQueue.mock.results[0].value
      await Promise.resolve()
    })

    expect(queue.leaveQueue).toHaveBeenCalledWith('q1')
    expect(window.location.pathname).toBe('/games/word-hunt')
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

  describe('a browser Back while queued', () => {
    beforeEach(() => {
      arriveOnWaiting()
    })

    it('asks before giving up the place in the queue', async () => {
      const onBackGesture = vi.fn()
      render(<Probe activeIntent={waitingIntent} onBackGesture={onBackGesture} />)

      await pressBack()

      expect(onBackGesture).toHaveBeenCalledTimes(1)
      expect(queue.leaveQueue).not.toHaveBeenCalled()
    })

    it('leaves the player on the waiting page, still queued, when they decline', async () => {
      render(<Probe activeIntent={waitingIntent} onBackGesture={vi.fn()} />)

      await pressBack()

      expect(window.location.pathname).toBe('/waiting')
      expect(queue.leaveQueue).not.toHaveBeenCalled()
      expect(queue.leaveQueueOnExit).not.toHaveBeenCalled()
    })

    it('asks again on a second back gesture rather than letting it escape', async () => {
      const onBackGesture = vi.fn()
      render(<Probe activeIntent={waitingIntent} onBackGesture={onBackGesture} />)

      await pressBack()
      await pressBack()

      expect(onBackGesture).toHaveBeenCalledTimes(2)
      expect(window.location.pathname).toBe('/waiting')
      expect(queue.leaveQueue).not.toHaveBeenCalled()
    })

    it('adds nothing to history just by being on the page, so reloads cannot pile up', () => {
      // Nothing is pushed on arrival, which is what makes a refresh while queued
      // cost nothing: there is no entry to reload into and duplicate.
      const before = window.history.length

      render(<Probe activeIntent={waitingIntent} onBackGesture={vi.fn()} />)

      expect(window.history.length).toBe(before)
      expect(window.location.pathname).toBe('/waiting')
    })

    it('leaves no stray waiting entry behind on an ordinary exit', async () => {
      // Stop looking, with no back gesture involved. Anything the page pushed on
      // arrival is still underneath at this point, and Back would land on it.
      const { rerender } = render(<Probe activeIntent={waitingIntent} onBackGesture={vi.fn()} />)

      rerender(<Probe activeIntent={null} onBackGesture={vi.fn()} />)
      act(() => navigateOutOfWaiting())

      await pressBack()

      expect(window.location.pathname).toBe('/games/word-hunt')
    })

    it('leaves no stray waiting entry behind once the player confirms', async () => {
      const { rerender } = render(<Probe activeIntent={waitingIntent} onBackGesture={vi.fn()} />)

      await pressBack()
      // Confirmed: the leave lands, the intent clears, and the page routes out.
      rerender(<Probe activeIntent={null} onBackGesture={vi.fn()} />)
      act(() => navigateOutOfWaiting())
      expect(window.location.pathname).toBe('/games/word-hunt')

      await pressBack()

      // Back must not drop the player onto a waiting page they have already left.
      expect(window.location.pathname).toBe('/games/word-hunt')
    })

    it('lets a matched player back out without being asked', async () => {
      // The queue place is already spent: there is nothing left to lose.
      const onBackGesture = vi.fn()
      render(<Probe activeIntent={{ queueId: 'q1', status: 'MATCHED' }} onBackGesture={onBackGesture} />)

      await pressBack()

      expect(onBackGesture).not.toHaveBeenCalled()
      expect(window.location.pathname).toBe('/games/word-hunt')
    })

    it('still leaves without asking when the app itself routes away', async () => {
      // Stop looking, or the dead-end guard: the app navigating is not a gesture to
      // second-guess, and the sheet has either been answered already or never applied.
      const onBackGesture = vi.fn()
      render(<Probe activeIntent={waitingIntent} onBackGesture={onBackGesture} />)

      act(() => navigateTo('/games/word-hunt'))

      expect(onBackGesture).not.toHaveBeenCalled()
      expect(queue.leaveQueue).toHaveBeenCalledWith('q1')
    })
  })
})
