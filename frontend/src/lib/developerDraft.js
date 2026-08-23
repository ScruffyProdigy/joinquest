export const DEVELOPER_DRAFT_KEY = 'lobby.developerRegistrationDraft'

/** Keep an in-progress game registration around while a guest signs up. */
export function saveRegistrationDraft(fields) {
  try {
    sessionStorage.setItem(DEVELOPER_DRAFT_KEY, JSON.stringify(fields))
  } catch {
    // Storage can be full or blocked — losing the draft beats breaking the form.
  }
}

/** Read a saved registration draft, or null when there isn't a usable one. */
export function readRegistrationDraft() {
  try {
    const raw = sessionStorage.getItem(DEVELOPER_DRAFT_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export function clearRegistrationDraft() {
  try {
    sessionStorage.removeItem(DEVELOPER_DRAFT_KEY)
  } catch {
    // Nothing to do — a stale draft is harmless.
  }
}
