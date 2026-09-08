/**
 * A visitor with no name and avatar is turned away by the backend and shown the
 * identity picker. Their intent — "play with friends, on this mode" — would otherwise
 * be lost, leaving them staring at the catalog after picking a name (JQ-131).
 *
 * Held in sessionStorage so it survives a sign-in round trip as well as the picker,
 * and keyed by game and mode so only the row the visitor actually pressed resumes.
 */
const KEY = 'lobby:pending-group-intent'

export function rememberGroupIntent(gameId, modeId) {
  try {
    window.sessionStorage.setItem(KEY, JSON.stringify({ gameId, modeId }))
  } catch {
    // private mode or storage disabled: the visitor just presses the button again
  }
}

export function takeGroupIntentFor(gameId, modeId) {
  let raw = null
  try {
    raw = window.sessionStorage.getItem(KEY)
  } catch {
    return false
  }
  if (!raw) {
    return false
  }
  let intent
  try {
    intent = JSON.parse(raw)
  } catch {
    return false
  }
  if (intent?.gameId !== gameId || intent?.modeId !== modeId) {
    return false
  }
  try {
    window.sessionStorage.removeItem(KEY)
  } catch {
    // best effort
  }
  return true
}
