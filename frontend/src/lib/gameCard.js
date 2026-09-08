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

/**
 * Display labels for the catalog axes (JQ-162).
 *
 * The server's taxonomy queries are the contract; these are the strings the card
 * paints. They are duplicated rather than fetched because the catalog query
 * already returns the axis *ids* on every game, and a second round trip to turn
 * six known ids into six known words is not worth the request. An id with no
 * entry here renders as nothing rather than as a raw slug.
 */
const GENRE_LABELS = {
  action: 'Action',
  strategy: 'Strategy',
  deduction: 'Deduction',
  'words-trivia': 'Words & Trivia',
  'drawing-creative': 'Drawing & Creative',
  puzzle: 'Puzzle',
}

const DIFFICULTY_LABELS = {
  casual: 'Casual',
  involved: 'Involved',
  demanding: 'Demanding',
}

const SOCIAL_MODE_LABELS = {
  'free-for-all': 'Free-for-all',
  '1v1': '1v1',
  teams: 'Teams',
  'hidden-roles': 'Hidden roles',
  'co-op': 'Co-op',
}

function axisLabel(labels, value) {
  const id = String(value || '').trim()
  return labels[id] || null
}

/** The game's genre label, e.g. "Words & Trivia". Null when the developer has not set one. */
export function gameGenreLabel(game) {
  return axisLabel(GENRE_LABELS, game?.genre)
}

/** The game's declared difficulty floor, e.g. "Casual". Null when undeclared. */
export function gameDifficultyLabel(game) {
  return axisLabel(DIFFICULTY_LABELS, game?.difficulty)
}

/** A mode's social shape, e.g. "1v1". Null when the manifest does not declare one. */
export function modeSocialModeLabel(mode) {
  return axisLabel(SOCIAL_MODE_LABELS, mode?.socialMode)
}

/**
 * Chips describing a game, most identifying axis first: genre, then this mode's
 * social shape, then difficulty.
 *
 * Each chip answers a different question, so unlike the flat tag list it replaced
 * the row cannot say the same thing twice. Pass the mode when the surface is
 * about one mode (a room table); omit it on whole-game surfaces.
 */
export function gameAxisChips(game, mode) {
  return [gameGenreLabel(game), modeSocialModeLabel(mode), gameDifficultyLabel(game)].filter(Boolean)
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

/**
 * Format a session length the way the prototype's cards do: "12 min" below an
 * hour, "1h" and "1h 30m" above it. Null when there is no usable number.
 */
function durationLabel(minutes) {
  const value = Number(minutes)
  if (!Number.isFinite(value) || value <= 0) {
    return null
  }
  const whole = Math.round(value)
  if (whole < 60) {
    return `${whole} min`
  }
  const hours = Math.floor(whole / 60)
  const rest = whole % 60
  return rest === 0 ? `${hours}h` : `${hours}h ${rest}m`
}

/** One mode's session length, e.g. "12 min". Null when the mode declares none. */
export function modeDurationLabel(mode) {
  return durationLabel(mode?.typicalMinutes)
}

/**
 * Session-length pill label aggregated across active modes, e.g. "5–12 min".
 *
 * Duration is declared per mode — Word Hunt's Arena runs about 12 minutes and
 * its Duel about 5 — but the catalog card is about the whole game, so it shows
 * the spread the same way the player-range pill beside it does, and collapses to
 * a single value when the modes agree. Modes with no declared duration are left
 * out rather than counted as zero; a game where none declare one gets no pill.
 */
export function gameDurationLabel(modes) {
  const declared = (Array.isArray(modes) ? modes : [])
    .filter((mode) => mode?.status === 'active')
    .map((mode) => Number(mode?.typicalMinutes))
    .filter((minutes) => Number.isFinite(minutes) && minutes > 0)
  if (declared.length === 0) {
    return null
  }
  const min = Math.min(...declared)
  const max = Math.max(...declared)
  if (min === max) {
    return durationLabel(min)
  }
  // While both ends share a unit only the upper bound spells it out, so the pill
  // reads as one range rather than as two durations sharing a dash. Once they
  // differ ("40 min" to "1h 10m") both need their own.
  if (max < 60) {
    return `${Math.round(min)}–${durationLabel(max)}`
  }
  return `${durationLabel(min)}–${durationLabel(max)}`
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
    { key: 'genre', icon: null, label: gameGenreLabel(game) },
    { key: 'players', icon: 'players', label: gamePlayerRangeLabel(game?.modes) },
    { key: 'duration', icon: 'duration', label: gameDurationLabel(game?.modes) },
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
