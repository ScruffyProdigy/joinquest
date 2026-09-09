import { graphqlRequest } from './graphql'

/*
 * Web Push registration (JQ-198).
 *
 * The whole point of this module is to never claim a player is reachable when
 * they are not. Every capability answer here is derived from something that
 * actually exists -- a registered service worker, a live PushSubscription, a
 * server that has VAPID keys -- and never from "permission was granted once".
 */

export const PUSH_CAPABILITY_QUERY = `
  query PushCapability {
    pushCapability {
      reachable
      subscriptionCount
      publicKey
    }
  }
`

export const SAVE_PUSH_SUBSCRIPTION_MUTATION = `
  mutation SavePushSubscription($input: SavePushSubscriptionInput!) {
    savePushSubscription(input: $input) {
      reachable
      subscriptionCount
      publicKey
    }
  }
`

export const DELETE_PUSH_SUBSCRIPTION_MUTATION = `
  mutation DeletePushSubscription($endpoint: String!) {
    deletePushSubscription(endpoint: $endpoint) {
      reachable
      subscriptionCount
      publicKey
    }
  }
`

const SERVICE_WORKER_PATH = '/sw.js'

/** True when this browser has the APIs Web Push needs at all. */
export function isPushSupported() {
  return (
    typeof window !== 'undefined'
    && 'serviceWorker' in navigator
    && 'PushManager' in window
    && typeof Notification !== 'undefined'
  )
}

/**
 * True when the app is running as an installed app rather than a browser tab.
 *
 * This is the iOS gate: Safari delivers push only to a site added to the Home
 * Screen, so on iOS an uninstalled visit can never be made reachable no matter
 * what the player taps.
 */
export function isStandalone() {
  if (typeof window === 'undefined') {
    return false
  }
  // navigator.standalone is the iOS-specific signal; the media query is the
  // standard one every other platform reports.
  return (
    window.navigator?.standalone === true
    || window.matchMedia?.('(display-mode: standalone)')?.matches === true
  )
}

/** True for iOS/iPadOS, where installing is a prerequisite rather than a nicety. */
export function isIOS() {
  if (typeof navigator === 'undefined') {
    return false
  }
  const ua = navigator.userAgent || ''
  if (/iPad|iPhone|iPod/.test(ua)) {
    return true
  }
  // iPadOS 13+ reports a desktop Safari UA; touch points are what give it away.
  return /Macintosh/.test(ua) && (navigator.maxTouchPoints || 0) > 1
}

/** The browser's current permission, without prompting. */
export function notificationPermission() {
  if (typeof Notification === 'undefined') {
    return 'unsupported'
  }
  return Notification.permission
}

/**
 * Why this browser cannot be made reachable, or null when it can.
 *
 * Returned rather than thrown because the caller uses it to decide what to
 * *offer*, not to report a failure: a player on iOS Safari should be shown the
 * install instructions, not an error.
 */
export function pushBlockedReason() {
  if (!isPushSupported()) {
    // On iOS this is what an ordinary Safari tab looks like: PushManager is
    // simply absent until the site is installed.
    return isIOS() && !isStandalone() ? 'needs-install' : 'unsupported'
  }
  if (isIOS() && !isStandalone()) {
    return 'needs-install'
  }
  if (notificationPermission() === 'denied') {
    // Sticky. Nothing the page does can re-prompt; only the player can, in
    // browser settings.
    return 'denied'
  }
  return null
}

/** Registers the service worker, returning its registration or null. */
export async function ensureServiceWorker() {
  if (!isPushSupported()) {
    return null
  }
  try {
    // Reuse an existing registration rather than re-registering on every call;
    // register() is idempotent but returns a promise that can outlive the page.
    const existing = await navigator.serviceWorker.getRegistration(SERVICE_WORKER_PATH)
    if (existing) {
      return existing
    }
    return await navigator.serviceWorker.register(SERVICE_WORKER_PATH, { scope: '/' })
  } catch {
    return null
  }
}

/** Fetches the server's view of this player's reachability. */
export async function fetchPushCapability() {
  const data = await graphqlRequest(PUSH_CAPABILITY_QUERY)
  return data.pushCapability
}

/**
 * VAPID keys travel as base64url text but subscribe() wants raw bytes.
 */
export function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = window.atob(base64)
  const output = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i += 1) {
    output[i] = raw.charCodeAt(i)
  }
  return output
}

function serializeSubscription(subscription) {
  const json = subscription.toJSON()
  return {
    endpoint: json.endpoint,
    p256dh: json.keys?.p256dh ?? '',
    auth: json.keys?.auth ?? '',
  }
}

/**
 * Asks for permission and registers this browser for match notifications.
 *
 * MUST be called from a user gesture -- browsers reject the prompt otherwise,
 * and a denial is sticky, so it cannot be spent casually. The acceptance
 * criteria are explicit that showing the affordance and spending the prompt are
 * separate things: the control is visible from the moment the player joins the
 * queue, and only pressing it gets here.
 *
 * Returns the server's capability snapshot, or a { blocked } reason.
 */
export async function enablePushNotifications() {
  const blocked = pushBlockedReason()
  if (blocked) {
    return { blocked }
  }

  // Ask the server first. If this deployment has no VAPID keys there is
  // nothing to subscribe to, and spending the player's one permission prompt
  // on a subscription nobody can send to is unrecoverable -- the denial
  // persists long after the config is fixed.
  const capability = await fetchPushCapability()
  if (!capability?.publicKey) {
    return { blocked: 'unsupported' }
  }

  const registration = await ensureServiceWorker()
  if (!registration) {
    return { blocked: 'unsupported' }
  }

  const permission = await Notification.requestPermission()
  if (permission !== 'granted') {
    return { blocked: permission === 'denied' ? 'denied' : 'dismissed' }
  }

  let subscription = await registration.pushManager.getSubscription()
  if (subscription) {
    // An existing subscription may predate a VAPID key rotation, in which case
    // it is bound to a key we can no longer sign with. Cheaper to drop and
    // re-create than to detect.
    const existingKey = subscription.options?.applicationServerKey
    if (existingKey && !applicationServerKeyMatches(existingKey, capability.publicKey)) {
      await subscription.unsubscribe()
      subscription = null
    }
  }

  if (!subscription) {
    subscription = await registration.pushManager.subscribe({
      // Required to be true by every browser: a push that shows no
      // notification is not allowed.
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(capability.publicKey),
    })
  }

  const data = await graphqlRequest(SAVE_PUSH_SUBSCRIPTION_MUTATION, {
    input: serializeSubscription(subscription),
  })
  return data.savePushSubscription
}

/** Compares the key a subscription was created with against the current one. */
export function applicationServerKeyMatches(existingKey, publicKey) {
  try {
    const current = urlBase64ToUint8Array(publicKey)
    const existing = new Uint8Array(existingKey)
    if (existing.length !== current.length) {
      return false
    }
    return existing.every((byte, index) => byte === current[index])
  } catch {
    return false
  }
}

/**
 * Turns notifications off for this browser: unsubscribes locally and drops the
 * row server-side.
 *
 * Both halves matter. Unsubscribing alone would leave the server believing the
 * player is reachable, and the seat-hold tiering keys off that being honest.
 */
export async function disablePushNotifications() {
  if (!isPushSupported()) {
    return null
  }
  const registration = await ensureServiceWorker()
  const subscription = await registration?.pushManager?.getSubscription()
  const endpoint = subscription?.endpoint

  if (subscription) {
    try {
      await subscription.unsubscribe()
    } catch {
      // Already gone as far as the browser is concerned; the server row is
      // still ours to clear.
    }
  }

  if (!endpoint) {
    return null
  }
  const data = await graphqlRequest(DELETE_PUSH_SUBSCRIPTION_MUTATION, { endpoint })
  return data.deletePushSubscription
}

/**
 * Drops this browser's local subscription without calling the server.
 *
 * Used on logout, where the server clears its own rows as part of the logout
 * mutation and the session is already gone, so a DELETE call would just fail
 * as unauthenticated.
 */
export async function forgetLocalPushSubscription() {
  if (!isPushSupported()) {
    return
  }
  try {
    const registration = await navigator.serviceWorker.getRegistration(SERVICE_WORKER_PATH)
    const subscription = await registration?.pushManager?.getSubscription()
    await subscription?.unsubscribe()
  } catch {
    // Best effort: never block a sign-out on it.
  }
}

/**
 * Re-registers after the push service rotates a subscription behind our back,
 * which the service worker reports via pushsubscriptionchange.
 */
export async function resubscribeAfterChange() {
  const registration = await ensureServiceWorker()
  if (!registration) {
    return null
  }
  const capability = await fetchPushCapability()
  if (!capability?.publicKey) {
    return null
  }
  const subscription = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(capability.publicKey),
  })
  const data = await graphqlRequest(SAVE_PUSH_SUBSCRIPTION_MUTATION, {
    input: serializeSubscription(subscription),
  })
  return data.savePushSubscription
}
