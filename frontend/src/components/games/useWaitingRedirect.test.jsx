import { render } from '@testing-library/react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useWaitingRedirect } from './useWaitingRedirect'

function Probe({ activeIntent }) {
  useWaitingRedirect(activeIntent)
  return null
}

describe('useWaitingRedirect', () => {
  beforeEach(() => {
    sessionStorage.clear()
    window.history.replaceState(null, '', '/')
  })

  it('sends a queued player to the waiting page', () => {
    window.history.replaceState(null, '', '/games/word-hunt')
    render(<Probe activeIntent={{ queueId: 'q1', status: 'WAITING' }} />)

    expect(window.location.pathname).toBe('/waiting')
    expect(sessionStorage.getItem('lobby.waitingReturnPath')).toBe('/games/word-hunt')
  })

  it('replaces rather than pushes, so Back off the waiting page is not a loop', () => {
    window.history.replaceState(null, '', '/games/word-hunt')
    const push = vi.spyOn(window.history, 'pushState')
    render(<Probe activeIntent={{ queueId: 'q1', status: 'WAITING' }} />)

    expect(push).not.toHaveBeenCalled()
    push.mockRestore()
  })

  it('leaves other intents alone', () => {
    render(<Probe activeIntent={{ queueId: 'q1', status: 'MATCHED' }} />)
    expect(window.location.pathname).toBe('/')

    render(<Probe activeIntent={null} />)
    expect(window.location.pathname).toBe('/')
  })

  it('does nothing when already on the waiting page', () => {
    window.history.replaceState(null, '', '/waiting')
    render(<Probe activeIntent={{ queueId: 'q1', status: 'WAITING' }} />)

    expect(window.location.pathname).toBe('/waiting')
    expect(sessionStorage.getItem('lobby.waitingReturnPath')).toBeNull()
  })
})
