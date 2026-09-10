const SNOOZE_KEY = 'lobby.installPromptSnoozedUntil'
const SNOOZE_DAYS = 7

/*
 * Remembers a dismissed install prompt so the player is asked again later
 * rather than never or every time.
 */

/** True when the player has dismissed the prompt recently. */
export function isInstallPromptSnoozed(now = Date.now()) {
  try {
    const until = Number(window.localStorage.getItem(SNOOZE_KEY))
    return Number.isFinite(until) && until > now
  } catch {
    // Private mode and blocked site data both throw on read.
    return false
  }
}

/** Records a dismissal. */
export function snoozeInstallPrompt(now = Date.now()) {
  try {
    window.localStorage.setItem(SNOOZE_KEY, String(now + SNOOZE_DAYS * 24 * 60 * 60 * 1000))
  } catch {
    // Nothing to do: the prompt reappears next time, which is the safe side.
  }
}

/** Clears the snooze, so a player who installs is not asked again by accident. */
export function clearInstallPromptSnooze() {
  try {
    window.localStorage.removeItem(SNOOZE_KEY)
  } catch {
    // Ignored.
  }
}
