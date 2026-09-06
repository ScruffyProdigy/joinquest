/**
 * A backend `identity required` rejection means the player tried to join a
 * queue, create a room, or sit at a table before finishing the identity prompt
 * — they are missing a name, an avatar, or both. The answer is the prompt
 * itself, not a raw GraphQL error, so the GraphQL client raises this signal and
 * the app shell opens the picker.
 *
 * Same-tab only — this is a UI signal, not the cross-tab auth broadcast.
 */
const listeners = new Set()

export function notifyIdentityRequired() {
  for (const listener of [...listeners]) {
    listener()
  }
}

export function onIdentityRequired(callback) {
  listeners.add(callback)
  return () => {
    listeners.delete(callback)
  }
}
