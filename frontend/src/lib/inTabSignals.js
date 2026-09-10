/*
 * In-tab attention signals for a player whose tab is open but not in front of
 * them: title flash, favicon badge, chime.
 *
 * A DESKTOP SUPPLEMENT, not a fallback for players who declined push. On mobile
 * there is no tab strip to flash, and iOS Safari suspends JS in a backgrounded
 * tab so the chime never fires. Do not promise a player these will reach them.
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

/** Flashes the document title. Idempotent: two calls do not stack timers. */
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
  // Flip immediately; a full interval of nothing looks broken.
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
 * Draws a dot over the favicon. Rendered rather than shipped as a second asset,
 * so it sits on whatever the current favicon is.
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
      // A cross-origin favicon taints the canvas. The other signals still fire.
    }
  }
  // Same-origin, so the canvas stays exportable.
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
 * Plays a short chime, synthesised so there is no audio file to fetch first.
 *
 * Autoplay policy requires a prior interaction with the page; every path here
 * has one. Best-effort, and never throws.
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
    // A fifth apart: reads as a notification, not an error.
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
    // Browsers cap how many contexts a page may hold.
    setTimeout(() => context.close?.(), 1200)
    return true
  } catch {
    return false
  }
}

/**
 * Fires every in-tab signal and returns their teardown. Bundled so a caller
 * cannot leave the title flashing after the player has come back.
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
