import { graphqlRequest } from './graphql'
import { isTabVisible } from './tabVisibility'

const SET_DOCUMENT_VISIBILITY_MUTATION = `
  mutation SetDocumentVisibility($documentId: ID!, $visible: Boolean!) {
    setDocumentVisibility(documentId: $documentId, visible: $visible)
  }
`

/**
 * One id per page load, shared by every report this document sends, so the server
 * replaces this document's answer instead of collecting a row per tab switch.
 *
 * Deliberately not persisted: a reload is a new document, and the old one's row is
 * cleared when its sockets close.
 */
let documentId = null
let stop = null

function newDocumentId() {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID()
  }
  // Older Safari has crypto but not randomUUID. The id only has to be unique
  // among one user's open documents, so this is enough.
  return `${Date.now().toString(16)}-${Math.random().toString(16).slice(2)}`
}

function report(visible) {
  // Best-effort: this is telemetry about attention, not something the player is
  // waiting on. A failed report just leaves them reading as present, which is
  // what happened before any of this existed.
  graphqlRequest(SET_DOCUMENT_VISIBILITY_MUTATION, { documentId, visible }).catch(() => {})
}

/**
 * Tell the server when this tab moves in and out of the foreground.
 *
 * The server cannot work this out for itself: a backgrounded tab holds its
 * websockets open exactly like a foregrounded one, so "connected" and "watching"
 * look identical from the outside. Matchmaking needs the difference, to avoid
 * starting a game for somebody who is not there to play it.
 *
 * Idempotent — several entry points import this and only the first call binds.
 */
export function startVisibilityReporting() {
  if (stop || typeof document === 'undefined') {
    return
  }
  documentId = newDocumentId()
  const handler = () => report(isTabVisible())
  document.addEventListener('visibilitychange', handler)
  stop = () => document.removeEventListener('visibilitychange', handler)
}

export function stopVisibilityReporting() {
  if (stop) {
    stop()
    stop = null
  }
}

/** Test seam: drops the listener and the id so each case starts clean. */
export function __resetVisibilityReporterForTests() {
  stopVisibilityReporting()
  documentId = null
}
