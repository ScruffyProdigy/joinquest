import { navigateTo } from './usePathname'

/** The waiting route, and the path to send the player back to when they leave it. */

export const WAITING_PATH = '/waiting'

const RETURN_KEY = 'lobby.waitingReturnPath'

export function parseWaitingRoute(pathname = window.location.pathname) {
  return /^\/waiting\/?$/.test(pathname)
}

/** Only a same-origin path comes back out: `//evil.example` is protocol-relative and
 * would navigate off-site. */
function safeReturnPath(value) {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) {
    return '/'
  }
  // Returning to the waiting page from itself is a loop, not a return.
  return parseWaitingRoute(value.split(/[?#]/)[0]) ? '/' : value
}

export function waitingReturnPath() {
  try {
    return safeReturnPath(sessionStorage.getItem(RETURN_KEY))
  } catch {
    return '/'
  }
}

/** sessionStorage, not history state: it has to survive a reload. */
export function rememberWaitingReturnPath(pathname = window.location.pathname) {
  try {
    sessionStorage.setItem(RETURN_KEY, safeReturnPath(pathname))
  } catch {
    // Storage unavailable (private mode); the exit falls back to the catalog.
  }
}

export function navigateToWaiting({ replace = false } = {}) {
  rememberWaitingReturnPath()
  navigateTo(WAITING_PATH, { replace })
}

/** Every exit from the waiting page. A formed match gets its own launch step in JQ-136. */
export function navigateOutOfWaiting({ replace = true } = {}) {
  const path = waitingReturnPath()
  try {
    sessionStorage.removeItem(RETURN_KEY)
  } catch {
    // Nothing to clean up.
  }
  navigateTo(path, { replace })
}
