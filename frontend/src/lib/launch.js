/** The launch moment between a match forming and the player entering the game (JQ-136). */

/** Long enough to read "You're in!", short enough not to feel like a stall. */
export const MATCH_FOUND_BEAT_MS = 1500

/** Seconds on the clock before the player is carried into the game. */
export const LAUNCH_COUNTDOWN_SECONDS = 5

/**
 * Entering the game leaves the lobby entirely, so this is a document navigation
 * rather than a route change. It lives here so the countdown can be tested without
 * a real navigation.
 */
export function navigateToLaunchUrl(url) {
  if (!url) {
    return
  }
  window.location.assign(url)
}
