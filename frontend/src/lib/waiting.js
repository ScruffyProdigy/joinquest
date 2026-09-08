import { navigateTo } from './usePathname'

/**
 * The queued state is a dedicated page, not a persistent banner (JQ-125, decided
 * 2026-09-07). This module owns the route and the one piece of state the page
 * needs beyond the intent itself: where the player came from, so leaving the
 * queue puts them back rather than dumping them on the catalog.
 */

export const WAITING_PATH = '/waiting'

const RETURN_KEY = 'lobby.waitingReturnPath'

export function parseWaitingRoute(pathname = window.location.pathname) {
  return /^\/waiting\/?$/.test(pathname)
}

/**
 * Only a same-origin path ever comes back out. A stored value is attacker-shaped
 * input in the general case, and `//evil.example` is a protocol-relative URL that
 * navigateTo would happily follow off-site.
 */
function safeReturnPath(value) {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) {
    return '/'
  }
  // Returning to the waiting page from the waiting page is a loop, not a return.
  return parseWaitingRoute(value.split(/[?#]/)[0]) ? '/' : value
}

export function waitingReturnPath() {
  try {
    return safeReturnPath(sessionStorage.getItem(RETURN_KEY))
  } catch {
    return '/'
  }
}

/**
 * sessionStorage rather than history state: a reload while queued has to land on
 * the waiting page with a usable way out, and history state does not survive one.
 */
export function rememberWaitingReturnPath(pathname = window.location.pathname) {
  try {
    sessionStorage.setItem(RETURN_KEY, safeReturnPath(pathname))
  } catch {
    // Storage can be unavailable (private mode); the exit falls back to the catalog.
  }
}

export function navigateToWaiting({ replace = false } = {}) {
  rememberWaitingReturnPath()
  navigateTo(WAITING_PATH, { replace })
}

/**
 * Every exit from the waiting page goes through here: the player stopped looking,
 * or a match formed. JQ-136 gives the match-formed case a launch step of its own —
 * until it exists, both exits go back where the player came from, where the
 * ready-to-play banner still carries the launch link.
 */
export function navigateOutOfWaiting({ replace = true } = {}) {
  const path = waitingReturnPath()
  try {
    sessionStorage.removeItem(RETURN_KEY)
  } catch {
    // Nothing to clean up if storage is unavailable.
  }
  navigateTo(path, { replace })
}
