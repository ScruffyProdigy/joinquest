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

/**
 * Undo a browser Back that has already left the waiting page, so the player can be
 * asked before their queue place is spent. Pushing rather than pretending keeps the
 * URL honest, and it costs no history depth: the pop removed an entry, this puts one
 * back. The remembered return path is deliberately left alone — the player has not
 * gone anywhere, so where they came from has not changed.
 */
export function restoreWaitingRoute() {
  // Already back on the page (two waiting entries in a row) — nothing to undo, and
  // pushing anyway would be the history leak this design exists to avoid.
  if (parseWaitingRoute()) {
    return
  }
  navigateTo(WAITING_PATH)
}

/** Every exit from the waiting page. A formed match is not one: it stays here for
 * the launch step (JQ-136) and leaves by entering the game. */
export function navigateOutOfWaiting({ replace = true } = {}) {
  const path = waitingReturnPath()
  try {
    sessionStorage.removeItem(RETURN_KEY)
  } catch {
    // Nothing to clean up.
  }
  navigateTo(path, { replace })
}
