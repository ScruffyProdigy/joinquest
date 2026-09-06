/**
 * Who is looking at the page, and how much we know about them.
 *
 * Two independent questions, deliberately kept apart:
 *   - which tier they are (no session / guest / full account)
 *   - whether their profile is filled in (name and avatar)
 * They cross: a brand-new email signup is a full account with no profile yet.
 */

/** No session at all — browsing without having identified themselves. */
export const VIEWER_PASSIVE = 'passive'
/** Has a session, but no email or social login behind it. */
export const VIEWER_GUEST = 'guest'
/** A real account they can come back to. */
export const VIEWER_MEMBER = 'member'

export function viewerTier(user) {
  if (!user) {
    return VIEWER_PASSIVE
  }
  return user.isGuest ? VIEWER_GUEST : VIEWER_MEMBER
}

/**
 * True once the player has picked a name. There is no placeholder to see
 * through — the backend leaves displayName null until they choose one.
 */
export function hasChosenDisplayName(user) {
  return Boolean(user?.displayName?.trim())
}

export function hasChosenAvatar(user) {
  return Boolean(user?.avatarKey?.trim())
    || Boolean(user?.avatarUrl?.trim())
    || user?.avatarSource === 'SPIRIT_ANIMAL'
}

/** The name to greet someone by, or '' when we do not have one yet. */
export function chosenDisplayName(user) {
  return user?.displayName?.trim() ?? ''
}

/**
 * The single trigger for the identity prompt. Moving the prompt to join time
 * means changing where this is called, not what it means.
 */
export function needsIdentity(user) {
  return !user || !hasChosenAvatar(user) || !hasChosenDisplayName(user)
}
