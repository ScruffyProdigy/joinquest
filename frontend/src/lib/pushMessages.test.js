import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  MATCH_READY,
  NOTIFICATION_CLICK,
  SUBSCRIPTION_CHANGED,
  listenForPushMessages,
} from './pushMessages'
import * as push from './push'
import * as signals from './inTabSignals'
import { mockPushSupported, mockPushUnsupported } from '../test/setup'

/** Delivers a message the way navigator.serviceWorker does. */
function emit(data) {
  const handler = navigator.serviceWorker.addEventListener.mock.calls
    .filter(([type]) => type === 'message')
    .map(([, fn]) => fn)
    .pop()
  handler?.({ data })
}

describe('listenForPushMessages', () => {
  beforeEach(() => {
    mockPushSupported()
    vi.spyOn(signals, 'startMatchReadySignals').mockImplementation(() => {})
    vi.spyOn(signals, 'stopMatchReadySignals').mockImplementation(() => {})
    vi.spyOn(push, 'resubscribeAfterChange').mockResolvedValue(null)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    mockPushUnsupported()
  })

  it('raises the in-tab signals for a push delivered to a visible tab', () => {
    const onMatchReady = vi.fn()
    listenForPushMessages({ onMatchReady })

    emit({ type: MATCH_READY, url: '/waiting' })

    // The worker suppresses the system notification for a visible tab, so
    // without this the push would be dropped entirely.
    expect(signals.startMatchReadySignals).toHaveBeenCalled()
    expect(onMatchReady).toHaveBeenCalledWith('/waiting')
  })

  it('clears the signals and routes when the player taps the notification', () => {
    const onNotificationClick = vi.fn()
    listenForPushMessages({ onNotificationClick })

    emit({ type: NOTIFICATION_CLICK, url: '/launch/abc' })

    expect(signals.stopMatchReadySignals).toHaveBeenCalled()
    expect(onNotificationClick).toHaveBeenCalledWith('/launch/abc')
  })

  it('does not route a push that landed on a visible tab', () => {
    const onNotificationClick = vi.fn()
    listenForPushMessages({ onMatchReady: vi.fn(), onNotificationClick })

    emit({ type: MATCH_READY, url: '/waiting' })

    // The page is live and updates over its own subscription. Navigating would
    // yank a page the player is actively using.
    expect(onNotificationClick).not.toHaveBeenCalled()
  })

  it('re-registers when the push service rotates the subscription', () => {
    listenForPushMessages()

    emit({ type: SUBSCRIPTION_CHANGED, oldEndpoint: 'https://push.example.com/old' })

    // Otherwise the stored endpoint stays dead and the player silently stops
    // being reachable.
    expect(push.resubscribeAfterChange).toHaveBeenCalled()
  })

  it('survives a failed re-registration', async () => {
    push.resubscribeAfterChange.mockRejectedValue(new Error('offline'))
    listenForPushMessages()

    expect(() => emit({ type: SUBSCRIPTION_CHANGED })).not.toThrow()
    await Promise.resolve()
  })

  it('ignores messages it does not own', () => {
    const onMatchReady = vi.fn()
    listenForPushMessages({ onMatchReady })

    emit({ type: 'some-other-library:event' })
    emit(undefined)

    expect(onMatchReady).not.toHaveBeenCalled()
    expect(signals.startMatchReadySignals).not.toHaveBeenCalled()
  })

  it('removes its listener on teardown', () => {
    const stop = listenForPushMessages()
    stop()
    expect(navigator.serviceWorker.removeEventListener).toHaveBeenCalledWith(
      'message',
      expect.any(Function),
    )
  })

  it('is a no-op on a browser without push', () => {
    mockPushUnsupported()
    const stop = listenForPushMessages({ onMatchReady: vi.fn() })
    expect(() => stop()).not.toThrow()
  })
})
