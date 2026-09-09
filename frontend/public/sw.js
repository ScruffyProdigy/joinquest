/*
 * JoinQuest service worker (JQ-198).
 *
 * Its only job is receiving push and routing the tap. It deliberately does NOT
 * cache anything: a stale shell would be far worse than a slow one here, and
 * /env.js is generated at build time, so precaching it would pin a deployment
 * to whichever API base URL it was built against.
 */

// Take over immediately rather than waiting for every tab to close. A player
// who just granted permission must be pushable now, not after their next
// full browser restart.
self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()))

const DEFAULT_TITLE = 'Your match is ready'
const DEFAULT_BODY = 'Tap to take your seat.'
const DEFAULT_URL = '/waiting'

function parsePayload(event) {
  if (!event.data) {
    return {}
  }
  try {
    return event.data.json() ?? {}
  } catch {
    // A push whose body is not our JSON is still a real signal that something
    // happened. Showing the default beats showing nothing -- and on some
    // platforms a push handler that shows no notification at all costs the
    // site its push permission.
    return {}
  }
}

self.addEventListener('push', (event) => {
  const payload = parsePayload(event)
  const title = payload.title || DEFAULT_TITLE
  const url = payload.url || DEFAULT_URL

  event.waitUntil(
    (async () => {
      // If a JoinQuest tab is already in front of the player, they can see the
      // waiting page changing on its own and a system notification is just
      // noise. The in-tab signals cover this case instead.
      const clients = await self.clients.matchAll({
        type: 'window',
        includeUncontrolled: true,
      })
      const visible = clients.some((client) => client.visibilityState === 'visible')
      if (visible) {
        for (const client of clients) {
          client.postMessage({ type: 'joinquest:match-ready', url })
        }
        return
      }

      await self.registration.showNotification(title, {
        body: payload.body || DEFAULT_BODY,
        icon: '/icons/icon-192.png',
        badge: '/icons/favicon-32.png',
        // Collapses retries: a second push for the same match replaces the
        // first rather than stacking identical alerts.
        tag: payload.tag || 'joinquest-match-ready',
        renotify: true,
        // The seat is being held on a deadline, so this must survive the
        // player glancing at their phone and looking away.
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

      // Focus an existing tab rather than opening a second one. Opening a new
      // window would leave the player with two JoinQuest tabs racing for the
      // same seat, and the acceptance criteria require the tap to land on the
      // launch step -- not on a fresh queue.
      for (const client of clients) {
        if ('focus' in client) {
          await client.focus()
          if ('navigate' in client) {
            try {
              await client.navigate(target)
            } catch {
              // Cross-origin or otherwise refused: the tab is focused, and the
              // message below lets the app route itself.
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
 * The push service can rotate a subscription without the page being open. When
 * that happens the stored endpoint is dead, so the page must re-register. The
 * SW cannot call the GraphQL API itself (it has no session cookie context we
 * want to depend on), so it tells whatever tab is open to do it.
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
