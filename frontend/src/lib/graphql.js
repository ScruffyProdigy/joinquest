import { getGraphQLUrl } from './env'
import { notifyIdentityRequired } from './identityPrompt'

export function isTransientServerError(message) {
  const text = (message || '').toLowerCase()
  return (
    text.includes('load failed')
    || text.includes('failed to fetch')
    || text.includes('networkerror')
    || text.includes('network request failed')
    || /api request failed \(502\)|api request failed \(503\)|api request failed \(504\)/.test(text)
  )
}

export function isAuthRequiredError(message) {
  return /authentication required/i.test(message || '')
}

/**
 * The caller has a session but has not finished picking a name and an avatar.
 * Deliberately distinct from isAuthRequiredError so the shell raises the
 * identity prompt rather than a sign-in error — matches ErrIdentityRequired in
 * backend/graph/auth_helpers.go.
 */
export function isIdentityRequiredError(message) {
  return /identity required/i.test(message || '')
}

/** Raises the identity prompt on the way out, so no call site has to remember to. */
function graphqlError(message) {
  if (isIdentityRequiredError(message)) {
    notifyIdentityRequired()
  }
  return new Error(message)
}

export async function graphqlRequest(query, variables = {}) {
  const response = await fetch(getGraphQLUrl(), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify({ query, variables }),
  })

  if (!response.ok) {
    let detail = ''
    try {
      const payload = await response.clone().json()
      detail = payload.errors?.[0]?.message || payload.error || ''
    } catch {
      // ignore parse errors
    }
    throw graphqlError(detail || `API request failed (${response.status})`)
  }

  const payload = await response.json()
  if (payload.errors?.length) {
    throw graphqlError(payload.errors[0]?.message || 'GraphQL request failed')
  }

  return payload.data
}
