import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  clearFaviconBadge,
  playChime,
  startSeatHeldSignals,
  startTitleFlash,
  stopSeatHeldSignals,
  stopTitleFlash,
} from './inTabSignals'

describe('title flash', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    document.title = 'JoinQuest'
  })

  afterEach(() => {
    stopTitleFlash()
    vi.useRealTimers()
  })

  it('shows the message immediately rather than after a full interval', () => {
    // A signal that takes a second to appear looks broken to anyone glancing
    // over as it starts.
    startTitleFlash('Match ready!')
    expect(document.title).toBe('Match ready!')
  })

  it('alternates between the message and the original title', () => {
    startTitleFlash('Match ready!')
    vi.advanceTimersByTime(1200)
    expect(document.title).toBe('JoinQuest')
    vi.advanceTimersByTime(1200)
    expect(document.title).toBe('Match ready!')
  })

  it('restores the original title when stopped', () => {
    startTitleFlash('Match ready!')
    vi.advanceTimersByTime(2400)
    stopTitleFlash()
    expect(document.title).toBe('JoinQuest')
  })

  it('does not keep flashing after being stopped', () => {
    startTitleFlash('Match ready!')
    stopTitleFlash()
    vi.advanceTimersByTime(6000)
    expect(document.title).toBe('JoinQuest')
  })

  it('does not stack timers when started twice', () => {
    // Two timers would fight over the title and leave it stuck on whichever
    // fired last, and the second start would capture 'Match ready!' as the
    // "original" title -- so stopping could never restore it.
    startTitleFlash('Match ready!')
    startTitleFlash('Match ready!')
    stopTitleFlash()
    vi.advanceTimersByTime(6000)
    expect(document.title).toBe('JoinQuest')
  })

  it('is safe to stop when it was never started', () => {
    expect(() => stopTitleFlash()).not.toThrow()
    expect(document.title).toBe('JoinQuest')
  })
})

describe('favicon badge', () => {
  beforeEach(() => {
    document.head.innerHTML = '<link rel="icon" href="/icons/favicon-32.png" />'
  })

  afterEach(() => {
    document.head.innerHTML = ''
  })

  it('restores the original favicon when cleared', () => {
    clearFaviconBadge()
    const link = document.querySelector('link[rel~="icon"]')
    expect(link.getAttribute('href')).toBe('/icons/favicon-32.png')
  })

  it('does not throw when the page has no favicon link', () => {
    document.head.innerHTML = ''
    expect(() => clearFaviconBadge()).not.toThrow()
  })
})

describe('chime', () => {
  afterEach(() => {
    delete window.AudioContext
    delete window.webkitAudioContext
  })

  it('reports failure rather than throwing when WebAudio is unavailable', () => {
    // jsdom has no AudioContext. An attention signal must never be able to
    // break the page it is trying to draw attention to.
    delete window.AudioContext
    delete window.webkitAudioContext
    expect(playChime()).toBe(false)
  })

  it('plays two notes and releases the audio context', () => {
    const stop = vi.fn()
    const start = vi.fn()
    const close = vi.fn()
    const connect = vi.fn()
    const oscillators = []

    window.AudioContext = vi.fn(() => ({
      currentTime: 0,
      destination: {},
      close,
      createOscillator: () => {
        const osc = { type: '', frequency: { value: 0 }, connect, start, stop }
        oscillators.push(osc)
        return osc
      },
      createGain: () => ({
        gain: {
          setValueAtTime: vi.fn(),
          exponentialRampToValueAtTime: vi.fn(),
        },
        connect,
      }),
    }))

    vi.useFakeTimers()
    expect(playChime()).toBe(true)
    expect(oscillators).toHaveLength(2)
    expect(start).toHaveBeenCalledTimes(2)
    expect(stop).toHaveBeenCalledTimes(2)

    // Browsers cap concurrent audio contexts; leaking one per notification
    // would eventually silence the chime entirely.
    vi.advanceTimersByTime(1500)
    expect(close).toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('swallows an AudioContext that throws', () => {
    window.AudioContext = vi.fn(() => {
      throw new Error('autoplay blocked')
    })
    expect(playChime()).toBe(false)
  })
})

describe('startSeatHeldSignals', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    // Order matters: assigning head.innerHTML removes the <title> element, so
    // setting the title first would leave it blank.
    document.head.innerHTML = '<link rel="icon" href="/icons/favicon-32.png" />'
    document.title = 'JoinQuest'
  })

  afterEach(() => {
    stopSeatHeldSignals()
    vi.useRealTimers()
    document.head.innerHTML = ''
  })

  it('returns a teardown that clears every signal at once', () => {
    const stop = startSeatHeldSignals('Match ready!')
    expect(document.title).toBe('Match ready!')

    stop()

    // A title left flashing after the player comes back is worse than never
    // having flashed, which is why teardown is handed to the caller rather
    // than left for them to assemble.
    expect(document.title).toBe('JoinQuest')
    vi.advanceTimersByTime(6000)
    expect(document.title).toBe('JoinQuest')
  })
})
