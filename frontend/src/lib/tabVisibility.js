/** True while the tab is in front of the player — false for a background tab. */
export function isTabVisible() {
  if (typeof document === 'undefined') {
    return true
  }
  return document.visibilityState !== 'hidden'
}

/** Tab visibility helper — re-sync state when the user returns to the tab. */
export function onTabVisible(callback) {
  if (typeof document === 'undefined') {
    return () => {}
  }
  const handler = () => {
    if (document.visibilityState === 'visible') {
      callback()
    }
  }
  document.addEventListener('visibilitychange', handler)
  return () => document.removeEventListener('visibilitychange', handler)
}
