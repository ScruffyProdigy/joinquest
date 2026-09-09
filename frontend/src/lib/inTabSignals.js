/*
 * In-tab attention signals (JQ-198): title flash, favicon badge, and a chime
 * for a player whose JoinQuest tab is open but not in front of them.
 *
 * These are a DESKTOP SUPPLEMENT, not a fallback tier for players who declined
 * push. The ticket is explicit about why: on mobile they mostly evaporate --
 * there is no visible tab strip for a title flash or a favicon badge to appear
 * in, and iOS Safari suspends JS in a backgrounded tab, so a timer-driven
 * sound never fires either. Nothing here should be described to a player as
 * "we'll let you know".
 */

const FAVICON_SIZE = 64
const BADGE_COLOR = '#2dd4bf'
const TITLE_FLASH_INTERVAL_MS = 1200

let originalTitle = null
let flashTimer = null
let originalFaviconHref = null

function faviconLink() {
  if (typeof document === 'undefined') {
    return null
  }
  return document.querySelector('link[rel~="icon"]')
}

/**
 * Starts alternating the document title so the tab strip catches the eye.
 * Idempotent: calling it twice does not stack two timers.
 */
export function startTitleFlash(message) {
  if (typeof document === 'undefined' || flashTimer) {
    return
  }
  originalTitle = document.title
  let showingMessage = false
  flashTimer = setInterval(() => {
    showingMessage = !showingMessage
    document.title = showingMessage ? message : originalTitle
  }, TITLE_FLASH_INTERVAL_MS)
  // Flip immediately -- waiting a full interval makes the signal look broken
  // to anyone glancing over right as it starts.
  document.title = message
  showingMessage = true
}

/** Restores the original document title. */
export function stopTitleFlash() {
  if (flashTimer) {
    clearInterval(flashTimer)
    flashTimer = null
  }
  if (typeof document !== 'undefined' && originalTitle !== null) {
    document.title = originalTitle
    originalTitle = null
  }
}

/**
 * Draws a dot over the favicon.
 *
 * Rendered rather than shipped as a second .ico so the badge can sit on
 * whatever the current favicon is, and so there is no second asset to keep in
 * sync with the icon set.
 */
export function showFaviconBadge() {
  const link = faviconLink()
  if (!link || typeof document === 'undefined') {
    return
  }
  if (originalFaviconHref === null) {
    originalFaviconHref = link.getAttribute('href')
  }

  const canvas = document.createElement('canvas')
  canvas.width = FAVICON_SIZE
  canvas.height = FAVICON_SIZE
  const context = canvas.getContext('2d')
  if (!context) {
    return
  }

  const image = new Image()
  image.onload = () => {
    context.drawImage(image, 0, 0, FAVICON_SIZE, FAVICON_SIZE)
    const radius = FAVICON_SIZE * 0.28
    const centre = FAVICON_SIZE - radius - 2
    context.beginPath()
    context.arc(centre, centre, radius, 0, Math.PI * 2)
    context.fillStyle = BADGE_COLOR
    context.fill()
    try {
      link.setAttribute('href', canvas.toDataURL('image/png'))
    } catch {
      // A tainted canvas (favicon served cross-origin) cannot be exported.
      // The title flash and chime still carry the signal.
    }
  }
  // Same-origin favicon, so this stays exportable.
  image.src = originalFaviconHref || '/icons/favicon-32.png'
}

/** Puts the original favicon back. */
export function clearFaviconBadge() {
  const link = faviconLink()
  if (link && originalFaviconHref !== null) {
    link.setAttribute('href', originalFaviconHref)
    originalFaviconHref = null
  }
}

/**
 * Plays a short chime.
 *
 * Synthesised with WebAudio rather than loaded as an audio file: it is a few
 * lines against a network fetch that would have to succeed before the sound
 * could play, at exactly the moment the player's attention is worth the most.
 *
 * Autoplay policy means this only works if the player has already interacted
 * with the page. Every path that reaches here has -- they pressed something to
 * join a queue -- but it is still best-effort and must never throw.
 */
export function playChime() {
  if (typeof window === 'undefined') {
    return false
  }
  const AudioContextClass = window.AudioContext || window.webkitAudioContext
  if (!AudioContextClass) {
    return false
  }
  try {
    const context = new AudioContextClass()
    const now = context.currentTime
    // Two notes a fifth apart: reads as a notification rather than an error.
    for (const [index, frequency] of [523.25, 783.99].entries()) {
      const oscillator = context.createOscillator()
      const gain = context.createGain()
      oscillator.type = 'sine'
      oscillator.frequency.value = frequency
      const start = now + index * 0.14
      // Ramped, not switched: an abrupt gain change clicks.
      gain.gain.setValueAtTime(0.0001, start)
      gain.gain.exponentialRampToValueAtTime(0.18, start + 0.02)
      gain.gain.exponentialRampToValueAtTime(0.0001, start + 0.35)
      oscillator.connect(gain)
      gain.connect(context.destination)
      oscillator.start(start)
      oscillator.stop(start + 0.4)
    }
    // Release the hardware; browsers cap how many contexts a page may hold.
    setTimeout(() => context.close?.(), 1200)
    return true
  } catch {
    return false
  }
}

/**
 * Fires every in-tab signal at once, and returns a function that clears them.
 *
 * Bundled deliberately: a title flash left running after the player returns is
 * worse than never flashing, so the caller is handed the cleanup rather than
 * being trusted to remember three separate teardowns.
 */
export function startMatchReadySignals(title = 'Your match is ready') {
  startTitleFlash(title)
  showFaviconBadge()
  playChime()
  return stopMatchReadySignals
}

/** Clears every in-tab signal. Safe to call when none are running. */
export function stopMatchReadySignals() {
  stopTitleFlash()
  clearFaviconBadge()
}
