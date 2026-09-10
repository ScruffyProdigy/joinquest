import { startMatchReadySignals, stopMatchReadySignals } from './inTabSignals'
import { isPushSupported, resubscribeAfterChange } from './push'

/*
 * Bridges the service worker's messages into the app.
 *
 * Without this the worker talks to nobody: a rotated subscription is never
 * repaired, and a push that arrives while the tab is visible is dropped
 * instead of raising the in-tab signals.
 */

export const MATCH_READY = 'joinquest:match-ready'
export const NOTIFICATION_CLICK = 'joinquest:notification-click'
export const SUBSCRIPTION_CHANGED = 'joinquest:push-subscription-changed'

/**
 * Starts listening. Returns a teardown.
 *
 * `onMatchReady` fires for a push delivered to a visible tab -- the page is
 * already live, so this is for attention, not navigation. `onNotificationClick`
 * fires when the player taps a notification and does need routing.
 */
export function listenForPushMessages({ onMatchReady, onNotificationClick } = {}) {
  if (!isPushSupported() || !navigator.serviceWorker?.addEventListener) {
    return () => {}
  }

  const handler = (event) => {
    const { type, url } = event.data ?? {}
    switch (type) {
      case MATCH_READY:
        // The tab is visible but may not be the window the player is looking
        // at. The page updates itself over its own subscription, so this only
        // needs to draw attention -- never navigate a page the player is using.
        startMatchReadySignals()
        onMatchReady?.(url)
        break
      case NOTIFICATION_CLICK:
        // They are here now, so clear anything still flashing.
        stopMatchReadySignals()
        onNotificationClick?.(url)
        break
      case SUBSCRIPTION_CHANGED:
        // Best-effort: the endpoint is already dead either way.
        resubscribeAfterChange().catch(() => {})
        break
      default:
        break
    }
  }

  navigator.serviceWorker.addEventListener('message', handler)
  return () => navigator.serviceWorker.removeEventListener('message', handler)
}
