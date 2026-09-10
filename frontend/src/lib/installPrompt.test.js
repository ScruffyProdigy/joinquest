import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  clearInstallPromptSnooze,
  isInstallPromptSnoozed,
  snoozeInstallPrompt,
} from './installPrompt'

const DAY = 24 * 60 * 60 * 1000

describe('installPrompt', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('is not snoozed before the player has dismissed anything', () => {
    expect(isInstallPromptSnoozed()).toBe(false)
  })

  it('stays snoozed for a week and then asks again', () => {
    const now = Date.now()
    snoozeInstallPrompt(now)

    expect(isInstallPromptSnoozed(now + DAY)).toBe(true)
    expect(isInstallPromptSnoozed(now + 6 * DAY)).toBe(true)
    // "Not now" is not "never" -- an iOS player who declines once should be
    // asked again, since installing is the only route to notifications there.
    expect(isInstallPromptSnoozed(now + 8 * DAY)).toBe(false)
  })

  it('can be cleared', () => {
    const now = Date.now()
    snoozeInstallPrompt(now)
    clearInstallPromptSnooze()
    expect(isInstallPromptSnoozed(now + DAY)).toBe(false)
  })

  it('treats unreadable storage as not snoozed', () => {
    // Private mode and blocked site data both throw. Showing the prompt is the
    // safe side of that failure.
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    expect(isInstallPromptSnoozed()).toBe(false)
  })

  it('does not throw when storage refuses a write', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota')
    })
    expect(() => snoozeInstallPrompt()).not.toThrow()
  })

  it('ignores a corrupted stored value', () => {
    window.localStorage.setItem('lobby.installPromptSnoozedUntil', 'not-a-number')
    expect(isInstallPromptSnoozed()).toBe(false)
  })
})
