import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  applicationServerKeyMatches,
  disablePushNotifications,
  enablePushNotifications,
  isIOS,
  isPushSupported,
  isStandalone,
  pushBlockedReason,
  urlBase64ToUint8Array,
} from './push'
import {
  makePushSubscription,
  mockAuthenticatedSession,
  mockPushSupported,
  mockPushUnsupported,
} from '../test/setup'

const TEST_PUBLIC_KEY =
  'BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM'

function setUserAgent(value, maxTouchPoints = 0) {
  Object.defineProperty(navigator, 'userAgent', {
    writable: true,
    configurable: true,
    value,
  })
  Object.defineProperty(navigator, 'maxTouchPoints', {
    writable: true,
    configurable: true,
    value: maxTouchPoints,
  })
}

const DESKTOP_UA = 'Mozilla/5.0 (X11; Linux x86_64) Chrome/120.0'
const IPHONE_UA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Safari/605.1'

describe('capability detection', () => {
  beforeEach(() => {
    mockPushUnsupported()
    setUserAgent(DESKTOP_UA)
    window.navigator.standalone = undefined
  })

  afterEach(() => {
    mockPushUnsupported()
  })

  it('reports push unsupported when the browser lacks the APIs', () => {
    expect(isPushSupported()).toBe(false)
    expect(pushBlockedReason()).toBe('unsupported')
  })

  it('reports push supported once the service worker and PushManager exist', () => {
    mockPushSupported()
    expect(isPushSupported()).toBe(true)
    expect(pushBlockedReason()).toBeNull()
  })

  it('treats a sticky denial as blocked rather than something to re-prompt', () => {
    mockPushSupported({ permission: 'denied' })
    expect(pushBlockedReason()).toBe('denied')
  })

  it('tells an iOS Safari tab it needs installing, not that it is unsupported', () => {
    // iOS Safari genuinely has no PushManager until the site is installed, so
    // the naive read is "unsupported" -- which would send the player away
    // instead of teaching them the Share-sheet gesture.
    setUserAgent(IPHONE_UA, 5)
    expect(pushBlockedReason()).toBe('needs-install')
  })

  it('stops asking an installed iOS app to install', () => {
    setUserAgent(IPHONE_UA, 5)
    window.navigator.standalone = true
    mockPushSupported()
    expect(isStandalone()).toBe(true)
    expect(pushBlockedReason()).toBeNull()
  })

  it('recognises iPadOS, which reports a desktop Safari user agent', () => {
    setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1', 5)
    expect(isIOS()).toBe(true)

    setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1', 0)
    expect(isIOS()).toBe(false)
  })
})

describe('urlBase64ToUint8Array', () => {
  it('decodes an unpadded base64url VAPID key', () => {
    const bytes = urlBase64ToUint8Array(TEST_PUBLIC_KEY)
    expect(bytes).toBeInstanceOf(Uint8Array)
    // An uncompressed P-256 point is 65 bytes and starts with 0x04.
    expect(bytes.length).toBe(65)
    expect(bytes[0]).toBe(0x04)
  })

  it('matches a key against itself and rejects a different one', () => {
    const bytes = urlBase64ToUint8Array(TEST_PUBLIC_KEY)
    expect(applicationServerKeyMatches(bytes.buffer, TEST_PUBLIC_KEY)).toBe(true)
    expect(applicationServerKeyMatches(new Uint8Array([1, 2, 3]).buffer, TEST_PUBLIC_KEY)).toBe(false)
  })
})

describe('enablePushNotifications', () => {
  beforeEach(() => {
    setUserAgent(DESKTOP_UA)
    window.navigator.standalone = undefined
    mockAuthenticatedSession()
  })

  afterEach(() => {
    mockPushUnsupported()
    vi.restoreAllMocks()
  })

  it('subscribes and registers the browser with the server', async () => {
    const { subscribe } = mockPushSupported({ requestResult: 'granted' })

    const result = await enablePushNotifications()

    expect(subscribe).toHaveBeenCalledTimes(1)
    // userVisibleOnly is mandatory: a silent push is rejected by every browser.
    expect(subscribe.mock.calls[0][0].userVisibleOnly).toBe(true)
    expect(result.reachable).toBe(true)
    expect(result.subscriptionCount).toBe(1)
  })

  it('does not spend the permission prompt when the server cannot send', async () => {
    // A denial is sticky and outlives the misconfiguration, so prompting here
    // would permanently cost the player notifications for no gain.
    mockAuthenticatedSession(undefined, {
      pushCapability: { reachable: false, subscriptionCount: 0, publicKey: null },
    })
    const { subscribe } = mockPushSupported({ requestResult: 'granted' })

    const result = await enablePushNotifications()

    expect(result).toEqual({ blocked: 'unsupported' })
    expect(Notification.requestPermission).not.toHaveBeenCalled()
    expect(subscribe).not.toHaveBeenCalled()
  })

  it('does not prompt at all on an uninstalled iOS tab', async () => {
    setUserAgent(IPHONE_UA, 5)
    mockPushSupported({ requestResult: 'granted' })

    const result = await enablePushNotifications()

    expect(result).toEqual({ blocked: 'needs-install' })
    expect(Notification.requestPermission).not.toHaveBeenCalled()
  })

  it('reports a denial distinctly from a dismissal', async () => {
    mockPushSupported({ requestResult: 'denied' })
    await expect(enablePushNotifications()).resolves.toEqual({ blocked: 'denied' })

    mockPushSupported({ requestResult: 'default' })
    await expect(enablePushNotifications()).resolves.toEqual({ blocked: 'dismissed' })
  })

  it('never subscribes when the player refuses', async () => {
    const { subscribe } = mockPushSupported({ requestResult: 'denied' })
    await enablePushNotifications()
    expect(subscribe).not.toHaveBeenCalled()
  })

  it('reuses an existing subscription created with the current key', async () => {
    const existing = makePushSubscription()
    existing.options = { applicationServerKey: urlBase64ToUint8Array(TEST_PUBLIC_KEY).buffer }
    const { subscribe } = mockPushSupported({ existingSubscription: existing })

    await enablePushNotifications()

    expect(subscribe).not.toHaveBeenCalled()
    expect(existing.unsubscribe).not.toHaveBeenCalled()
  })

  it('replaces a subscription bound to a rotated-away VAPID key', async () => {
    // Left in place it would be an endpoint the server can no longer sign for:
    // stored, counted as reachable, and silently undeliverable.
    const stale = makePushSubscription()
    stale.options = { applicationServerKey: new Uint8Array([9, 9, 9]).buffer }
    const { subscribe } = mockPushSupported({ existingSubscription: stale })
    subscribe.mockResolvedValueOnce(makePushSubscription('https://push.example.com/fresh'))

    await enablePushNotifications()

    expect(stale.unsubscribe).toHaveBeenCalledTimes(1)
    expect(subscribe).toHaveBeenCalledTimes(1)
  })
})

describe('disablePushNotifications', () => {
  beforeEach(() => {
    setUserAgent(DESKTOP_UA)
    mockAuthenticatedSession()
  })

  afterEach(() => {
    mockPushUnsupported()
  })

  it('unsubscribes locally and clears the server row', async () => {
    const existing = makePushSubscription()
    mockPushSupported({ existingSubscription: existing })

    const result = await disablePushNotifications()

    // Both halves matter: unsubscribing alone would leave the server believing
    // the player is still reachable.
    expect(existing.unsubscribe).toHaveBeenCalledTimes(1)
    expect(result.reachable).toBe(false)
    expect(result.subscriptionCount).toBe(0)
  })

  it('is a no-op when there was nothing subscribed', async () => {
    mockPushSupported({ existingSubscription: null })
    await expect(disablePushNotifications()).resolves.toBeNull()
  })

  it('is a no-op on a browser that cannot do push', async () => {
    mockPushUnsupported()
    await expect(disablePushNotifications()).resolves.toBeNull()
  })
})
