import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  WAITING_PATH,
  navigateOutOfWaiting,
  navigateToWaiting,
  parseWaitingRoute,
  rememberWaitingReturnPath,
  waitingReturnPath,
} from './waiting'

function setPath(pathname) {
  window.history.replaceState(null, '', pathname)
}

describe('waiting route', () => {
  beforeEach(() => {
    sessionStorage.clear()
    setPath('/')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('matches /waiting with or without a trailing slash', () => {
    expect(parseWaitingRoute('/waiting')).toBe(true)
    expect(parseWaitingRoute('/waiting/')).toBe(true)
    expect(parseWaitingRoute('/waiting/extra')).toBe(false)
    expect(parseWaitingRoute('/')).toBe(false)
    expect(parseWaitingRoute('/games/word-hunt')).toBe(false)
  })

  it('remembers where the player came from', () => {
    setPath('/games/word-hunt')
    rememberWaitingReturnPath()
    expect(waitingReturnPath()).toBe('/games/word-hunt')
  })

  it('falls back to the catalog when nothing was remembered', () => {
    expect(waitingReturnPath()).toBe('/')
  })

  it('refuses an off-site return path', () => {
    sessionStorage.setItem('lobby.waitingReturnPath', '//evil.example/steal')
    expect(waitingReturnPath()).toBe('/')

    sessionStorage.setItem('lobby.waitingReturnPath', 'https://evil.example')
    expect(waitingReturnPath()).toBe('/')
  })

  it('never returns to the waiting page itself', () => {
    setPath('/waiting')
    rememberWaitingReturnPath()
    expect(waitingReturnPath()).toBe('/')
  })

  it('survives sessionStorage being unavailable', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('denied')
    })
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied')
    })
    expect(() => rememberWaitingReturnPath('/games/word-hunt')).not.toThrow()
    expect(waitingReturnPath()).toBe('/')
  })

  it('pushes onto the waiting page and remembers the origin', () => {
    setPath('/games/word-hunt')
    navigateToWaiting()
    expect(window.location.pathname).toBe(WAITING_PATH)
    expect(waitingReturnPath()).toBe('/games/word-hunt')
  })

  it('replaces instead of pushing when asked', () => {
    setPath('/games/word-hunt')
    const replace = vi.spyOn(window.history, 'replaceState')
    navigateToWaiting({ replace: true })
    expect(replace).toHaveBeenCalled()
    expect(window.location.pathname).toBe(WAITING_PATH)
  })

  it('routes back out to the remembered page and forgets it', () => {
    setPath('/games/word-hunt')
    navigateToWaiting()
    navigateOutOfWaiting()
    expect(window.location.pathname).toBe('/games/word-hunt')
    expect(waitingReturnPath()).toBe('/')
  })
})
