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

export const VERIFY_PUSH_SUBSCRIPTION_MUTATION = `
  mutation VerifyPushSubscription($endpoint: String!) {
    verifyPushSubscription(endpoint: $endpoint) {
      sent
      capability {
        reachable
        subscriptionCount
        publicKey
      }
    }
  }
`

export const CONFIRM_PUSH_VERIFICATION_MUTATION = `
  mutation ConfirmPushVerification($token: String!) {
    confirmPushVerification(token: $token) {
      verified
    }
  }
`

const SERVICE_WORKER_PATH = '/sw.js'

/**
 * Fired on `window` once a verification push has been acked, so a pending
 * opt-in can finish the moment the round trip closes instead of on its next
 * poll.
 */
export const PUSH_VERIFIED_EVENT = 'joinquest:push-verified'

/**
 * How long to wait for the round trip before telling the player we cannot
 * reach them.
 *
 * Long enough for a slow push service, short enough that nobody is left
 * looking at a spinner deciding whether it is safe to close the tab -- the one
 * question this control exists to answer.
 */
export const VERIFICATION_TIMEOUT_MS = 12000

/** How often to re-ask the server while waiting, in case the ack came in via a
 * tab this one cannot hear from. */
export const VERIFICATION_POLL_MS = 1000

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
  // A worker predating verification would show the silent push as a
  // notification instead of acking it. Cheap insurance, once, on a press.
  try {
    await registration.update?.()
  } catch {
    // An update check that fails leaves the worker we already have.
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

  // Registering is a claim, not a capability. Prove it before the player acts
  // on it: this is the difference between "we think we can reach you" and a
  // fact recorded seconds ago, which is what decides whether leaving costs
  // them their place.
  return verifySubscription(subscription.endpoint, data.savePushSubscription)
}

/**
 * Runs the round trip: ask the backend to push this endpoint, then wait for the
 * service worker's ack to come back through the server.
 *
 * Returns the verified capability, or `{ blocked: 'unverified', capability }`
 * when nothing arrives. Never reports success on a timeout -- a silent
 * downgrade here is the failure mode the whole mechanism is built to avoid.
 */
export async function verifySubscription(endpoint, saved) {
  const data = await graphqlRequest(VERIFY_PUSH_SUBSCRIPTION_MUTATION, { endpoint })
  const verification = data.verifyPushSubscription

  if (!verification?.sent) {
    // Nothing left the building, so no ack is coming and waiting would only
    // spend the player's patience.
    return { blocked: 'unverified', capability: verification?.capability ?? saved }
  }

  const capability = await waitForVerification()
  if (!capability?.reachable) {
    return { blocked: 'unverified', capability: capability ?? saved }
  }
  return capability
}

/**
 * Waits for the server to agree this player is reachable.
 *
 * Two ways in, because the ack does not come back through this call: the tab
 * that received the push fires PUSH_VERIFIED_EVENT, and a poll covers the case
 * where that tab is a different one. Resolves with the capability, or null on
 * timeout.
 */
export function waitForVerification({
  timeoutMs = VERIFICATION_TIMEOUT_MS,
  pollMs = VERIFICATION_POLL_MS,
} = {}) {
  return new Promise((resolve) => {
    let settled = false

    const finish = (value) => {
      if (settled) {
        return
      }
      settled = true
      clearInterval(poll)
      clearTimeout(deadline)
      window.removeEventListener(PUSH_VERIFIED_EVENT, onVerified)
      resolve(value)
    }

    const check = async () => {
      try {
        const capability = await fetchPushCapability()
        if (capability?.reachable) {
          finish(capability)
        }
      } catch {
        // Keep waiting: a dropped poll is not an answer.
      }
    }

    const onVerified = () => {
      void check()
    }

    const poll = setInterval(check, pollMs)
    const deadline = setTimeout(() => finish(null), timeoutMs)
    window.addEventListener(PUSH_VERIFIED_EVENT, onVerified)
    void check()
  })
}

/**
 * Hands a verification token back to the server. Called by whichever tab the
 * service worker reached, which need not be the one that opted in.
 */
export async function confirmPushVerification(token) {
  const data = await graphqlRequest(CONFIRM_PUSH_VERIFICATION_MUTATION, { token })
  const verified = data.confirmPushVerification?.verified === true
  if (verified) {
    window.dispatchEvent(new CustomEvent(PUSH_VERIFIED_EVENT))
  }
  return verified
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

  // The rotation cleared the old proof server-side, and rightly: it was about
  // keys this endpoint no longer holds. Earn it again, quietly -- no player
  // pressed anything here.
  return verifySubscription(subscription.endpoint, data.savePushSubscription)
}
