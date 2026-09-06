/** Bump when replacing catalog/detail artwork so browsers skip stale caches. */
export const GAME_ARTWORK_VERSION = 1

function withArtworkVersion(url) {
  const trimmed = String(url || '').trim()
  if (!trimmed || trimmed.includes('?v=')) {
    return trimmed
  }
  return `${trimmed}?v=${GAME_ARTWORK_VERSION}`
}

/** Resolve a catalog game icon URL (required on every game). */
export function gameIconUrl(game) {
  const url = game?.iconUrl?.trim()
  if (!url) {
    throw new Error('game iconUrl is required')
  }
  return withArtworkVersion(url)
}

/** Art a card falls back to when a game ships without a hero of its own. */
export const DEFAULT_HERO_URL = '/games/default-hero.svg'

/**
 * Resolve the catalog card hero (4:3). Uses catalogHeroUrl when set, else heroUrl.
 *
 * A game with neither gets the shared placeholder rather than an exception: the
 * card is the whole catalog row, so one game missing art should read as a plain
 * card, not take the catalog down with it.
 */
export function gameCatalogHeroUrl(game) {
  const url = game?.catalogHeroUrl?.trim() || game?.heroUrl?.trim()
  return withArtworkVersion(url || DEFAULT_HERO_URL)
}

/** Resolve a catalog game hero banner URL for the detail page (16:9 slot). */
export function gameHeroUrl(game) {
  const url = game?.heroUrl?.trim()
  if (!url) {
    throw new Error('game heroUrl is required')
  }
  return withArtworkVersion(url)
}

/** Shareable detail page path, or null when the game has no slug. */
export function gamePagePath(game) {
  const slug = game?.slug?.trim()
  if (!slug) {
    return null
  }
  return `/games/${slug}`
}

/** Absolute URL for sharing a game detail page. */
export function gamePageShareUrl(game) {
  const path = gamePagePath(game)
  if (!path) {
    return null
  }
  if (typeof window !== 'undefined' && window.location?.origin) {
    return `${window.location.origin}${path}`
  }
  return path
}

/** Long description for the detail page, falling back to the catalog blurb. */
export function gameDetailDescription(game) {
  const long = game?.longDescription?.trim()
  if (long) {
    return long
  }
  return game?.shortDescription?.trim() || ''
}

/** Visible tag chips for catalog cards (max 3). */
export function gameTagChips(tags) {
  if (!Array.isArray(tags)) {
    return []
  }
  return tags.map((tag) => String(tag).trim()).filter(Boolean).slice(0, 3)
}

/** Genre/mode badge label from the first two tag chips, e.g. "Trivia · Party". Null with no tags. */
export function gameGenreModeLabel(tags) {
  const chips = gameTagChips(tags)
  if (chips.length === 0) {
    return null
  }
  return chips.slice(0, 2).join(' · ')
}

/**
 * Player-range pill label aggregated across active modes, e.g. "2–8" or "2".
 *
 * Bare numbers, because the pill carries a player glyph and repeating "players"
 * beside it just costs width on a narrow card.
 */
export function gamePlayerRangeLabel(modes) {
  const active = (Array.isArray(modes) ? modes : []).filter((mode) => mode?.status === 'active')
  if (active.length === 0) {
    return null
  }
  const min = Math.min(...active.map((mode) => mode.minPlayers))
  const max = Math.max(...active.map((mode) => mode.maxPlayers))
  if (min === max) {
    return String(min)
  }
  return `${min}–${max}`
}

/** Session-length pill label, e.g. "12 min". Null until a game carries a duration. */
export function gameDurationLabel(game) {
  const minutes = Number(game?.durationMinutes)
  if (!Number.isFinite(minutes) || minutes <= 0) {
    return null
  }
  return `${Math.round(minutes)} min`
}

/**
 * The card's metadata pills in their fixed order: genre, then player range, then
 * duration.
 *
 * Absent facts drop out of the row entirely rather than holding an empty slot,
 * so a catalog where no game has a duration yet reads as a shorter row instead of
 * a row of gaps. The order is fixed here rather than at the call site so cards
 * stay comparable down a column even when they carry different pills.
 */
export function gameCardMeta(game) {
  return [
    { key: 'genre', icon: null, label: gameGenreModeLabel(game?.tags) },
    { key: 'players', icon: 'players', label: gamePlayerRangeLabel(game?.modes) },
    { key: 'duration', icon: 'duration', label: gameDurationLabel(game) },
  ].filter((pill) => Boolean(pill.label))
}

/**
 * Live activity badge label. Prefers players in a session ("4 playing"); with nobody playing
 * but people queued it shows "2 waiting", which reads as an invitation to join rather than a
 * dead card. Null when the game is quiet.
 */
export function gameLiveActivityLabel(playerActivity) {
  const playing = Number(playerActivity?.playing) || 0
  if (playing > 0) {
    return `${playing} playing`
  }
  const queued = Number(playerActivity?.queued) || 0
  if (queued > 0) {
    return `${queued} waiting`
  }
  return null
}

// Wordmark placement constants. The prototype uses one inset and one height cap
// for every mark; only the anchor and width vary per game, so only those two
// travel with the art.
const TITLE_INSET_X_PCT = 5.5
const TITLE_INSET_Y_PCT = 6.8
const TITLE_MAX_HEIGHT_PCT = 26

const TITLE_ANCHORS = {
  'top-left': { top: `${TITLE_INSET_Y_PCT}%`, left: `${TITLE_INSET_X_PCT}%`, objectPosition: 'left top' },
  'top-center': { top: `${TITLE_INSET_Y_PCT}%`, left: '50%', translateX: true, objectPosition: 'center top' },
  'bottom-left': { bottom: `${TITLE_INSET_Y_PCT}%`, left: `${TITLE_INSET_X_PCT}%`, objectPosition: 'left bottom' },
  'bottom-center': { bottom: `${TITLE_INSET_Y_PCT}%`, left: '50%', translateX: true, objectPosition: 'center bottom' },
  'middle-center': { top: '50%', left: '50%', translateX: true, translateY: true, objectPosition: 'center' },
}

/**
 * Inline style placing a game's wordmark over its card art, or null when the game
 * has no usable mark.
 *
 * Width is a share of the *card*, not of the image, so the mark keeps its relative
 * weight as cards narrow — scaling a baked-in mark with the artwork instead would
 * leave most of them unreadable at phone width. The height cap keeps a tall lockup
 * from swallowing the card; `object-fit: contain` means hitting that cap shrinks
 * the mark rather than squashing it.
 */
export function gameTitleArtStyle(titleArt) {
  const url = titleArt?.url?.trim()
  const width = Number(titleArt?.widthPct)
  const anchor = TITLE_ANCHORS[titleArt?.anchor]
  if (!url || !anchor || !Number.isFinite(width) || width <= 0) {
    return null
  }
  const transform = [anchor.translateX ? 'translateX(-50%)' : '', anchor.translateY ? 'translateY(-50%)' : '']
    .filter(Boolean)
    .join(' ')
  return {
    position: 'absolute',
    top: anchor.top,
    bottom: anchor.bottom,
    left: anchor.left,
    width: `${Math.min(width, 100)}%`,
    maxHeight: `${TITLE_MAX_HEIGHT_PCT}%`,
    objectFit: 'contain',
    objectPosition: anchor.objectPosition,
    ...(transform ? { transform } : {}),
  }
}

/** Wordmark URL for a game's card, cache-busted like the rest of its art. */
export function gameTitleArtUrl(titleArt) {
  const url = titleArt?.url?.trim()
  return url ? withArtworkVersion(url) : null
}
