/*
 * JoinQuest service worker: receives push and routes the tap.
 *
 * It caches nothing. A stale shell would be worse than a slow one, and /env.js
 * is generated per deployment, so precaching it would pin the API base URL.
 */

// Take over immediately: a player who just granted permission must be
// pushable now, not after every tab has closed.
self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()))

const DEFAULT_TITLE = 'Your game is nearly ready'
const DEFAULT_BODY = 'Come back now to keep your spot.'
const DEFAULT_URL = '/waiting'

function parsePayload(event) {
  if (!event.data) {
    return {}
  }
  try {
    return event.data.json() ?? {}
  } catch {
    // Still a real signal. Show the default: on some platforms a push handler
    // that shows nothing at all costs the site its permission.
    return {}
  }
}

self.addEventListener('push', (event) => {
  const payload = parsePayload(event)
  const title = payload.title || DEFAULT_TITLE
  const url = payload.url || DEFAULT_URL

  event.waitUntil(
    (async () => {
      // A visible tab already shows the change, so a system notification is
      // noise. The page fires the in-tab signals instead.
      const clients = await self.clients.matchAll({
        type: 'window',
        includeUncontrolled: true,
      })
      const visible = clients.some((client) => client.visibilityState === 'visible')
      if (visible) {
        for (const client of clients) {
          client.postMessage({ type: 'joinquest:seat-held', url })
        }
        return
      }

      await self.registration.showNotification(title, {
        body: payload.body || DEFAULT_BODY,
        icon: '/icons/icon-192.png',
        badge: '/icons/favicon-32.png',
        // Collapses retries into one alert.
        tag: payload.tag || 'joinquest-seat-held',
        renotify: true,
        // The seat has a deadline, so this must survive a glance.
        requireInteraction: true,
        data: { url },
      })
    })(),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const target = event.notification.data?.url || DEFAULT_URL

  event.waitUntil(
    (async () => {
      const clients = await self.clients.matchAll({
        type: 'window',
        includeUncontrolled: true,
      })

      // Focus an existing tab; a second window would race the first for the
      // same seat.
      for (const client of clients) {
        if ('focus' in client) {
          await client.focus()
          if ('navigate' in client) {
            try {
              await client.navigate(target)
            } catch {
              // Refused. The tab is focused, and the message below lets the
              // app route itself.
            }
          }
          client.postMessage({ type: 'joinquest:notification-click', url: target })
          return
        }
      }

      if (self.clients.openWindow) {
        await self.clients.openWindow(target)
      }
    })(),
  )
})

/*
 * The push service can rotate a subscription while no page is open, leaving the
 * stored endpoint dead. The worker has no session to re-register with, so it
 * asks whatever tab is open to do it.
 */
self.addEventListener('pushsubscriptionchange', (event) => {
  event.waitUntil(
    (async () => {
      const clients = await self.clients.matchAll({
        type: 'window',
        includeUncontrolled: true,
      })
      for (const client of clients) {
        client.postMessage({
          type: 'joinquest:push-subscription-changed',
          oldEndpoint: event.oldSubscription?.endpoint ?? null,
        })
      }
    })(),
  )
})
