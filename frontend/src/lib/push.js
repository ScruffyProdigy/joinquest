import { graphqlRequest } from './graphql'

/*
 * Web Push registration.
 *
 * Every capability answer here comes from something that actually exists -- a
 * registered service worker, a live PushSubscription, a server with VAPID keys
 * -- never from "permission was granted once".
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
 * True when running as an installed app rather than a browser tab.
 *
 * The iOS gate: Safari delivers push only to a Home Screen install.
 */
export function isStandalone() {
  if (typeof window === 'undefined') {
    return false
  }
  // navigator.standalone is iOS-only; the media query covers everything else.
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
  // iPadOS 13+ reports a desktop Safari UA; touch points give it away.
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
 * Why this browser cannot be made reachable, or null when it can. The caller
 * uses it to decide what to offer -- iOS Safari gets install instructions,
 * not an error.
 */
export function pushBlockedReason() {
  if (!isPushSupported()) {
    // On iOS, PushManager is simply absent until the site is installed.
    return isIOS() && !isStandalone() ? 'needs-install' : 'unsupported'
  }
  if (isIOS() && !isStandalone()) {
    return 'needs-install'
  }
  if (notificationPermission() === 'denied') {
    // Sticky: only the player can undo this, in browser settings.
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
    // Reuse an existing registration rather than re-registering each call.
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

/** VAPID keys travel as base64url text; subscribe() wants raw bytes. */
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
 * MUST be called from a user gesture, or the browser rejects the prompt. A
 * denial is sticky, so this is only reached by a deliberate press.
 *
 * Returns the server's capability snapshot, or a { blocked } reason.
 */
export async function enablePushNotifications() {
  const blocked = pushBlockedReason()
  if (blocked) {
    return { blocked }
  }

  // Ask the server first: with no VAPID keys there is nothing to subscribe to,
  // and a denial would outlive the misconfiguration that caused it.
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
    // A subscription bound to a rotated-away key is silently undeliverable.
    const existingKey = subscription.options?.applicationServerKey
    if (existingKey && !applicationServerKeyMatches(existingKey, capability.publicKey)) {
      await subscription.unsubscribe()
      subscription = null
    }
  }

  if (!subscription) {
    subscription = await registration.pushManager.subscribe({
      // Required by every browser: a silent push is not allowed.
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(capability.publicKey),
    })
  }

  const data = await graphqlRequest(SAVE_PUSH_SUBSCRIPTION_MUTATION, {
    input: serializeSubscription(subscription),
  })
  return data.savePushSubscription
}

/** True when a subscription was created with the current VAPID key. */
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
 * Turns notifications off: unsubscribes locally and drops the server row.
 * Both halves matter, or the server still believes the player is reachable.
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
      // Already gone locally; the server row is still ours to clear.
    }
  }

  if (!endpoint) {
    return null
  }
  const data = await graphqlRequest(DELETE_PUSH_SUBSCRIPTION_MUTATION, { endpoint })
  return data.deletePushSubscription
}

/**
 * Drops the local subscription without calling the server. For logout, where
 * the server clears its own rows and the session is already gone.
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
    // Never block a sign-out on it.
  }
}

/** Re-registers after the push service rotates a subscription behind our back. */
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
