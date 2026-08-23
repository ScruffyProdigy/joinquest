import { graphqlRequest } from './graphql'
import { getApiBaseUrl } from './env'

function oauthBaseUrl() {
  const base = getApiBaseUrl()
  if (base) {
    return base.replace(/\/$/, '')
  }
  return ''
}

export function buildOAuthStartUrl(provider, { mode = 'signin', confirmMerge = false, next = null } = {}) {
  const slug = String(provider || '').trim().toLowerCase()
  const params = new URLSearchParams()
  if (mode === 'link') {
    params.set('mode', 'link')
  }
  if (confirmMerge) {
    params.set('confirm_merge', '1')
  }
  // An opaque destination key — the backend maps it to a fixed path, or ignores it.
  if (next) {
    params.set('next', next)
  }
  const query = params.toString()
  const path = `/auth/oauth/${encodeURIComponent(slug)}/start${query ? `?${query}` : ''}`
  const apiBase = oauthBaseUrl()
  return apiBase ? `${apiBase}${path}` : path
}

export function startOAuthSignIn(provider, next = null) {
  window.location.assign(buildOAuthStartUrl(provider, { mode: 'signin', next }))
}

export function startOAuthLink(provider, confirmMerge = false) {
  window.location.assign(buildOAuthStartUrl(provider, { mode: 'link', confirmMerge }))
}

const ENABLED_OAUTH_PROVIDERS_QUERY = `
  query EnabledOAuthProviders {
    enabledOAuthProviders
  }
`

const REMOVE_LINKED_IDENTITY_MUTATION = `
  mutation RemoveLinkedIdentity($identityId: ID!) {
    removeLinkedIdentity(identityId: $identityId) {
      id
    }
  }
`

export async function fetchEnabledOAuthProviders() {
  const data = await graphqlRequest(ENABLED_OAUTH_PROVIDERS_QUERY)
  return data.enabledOAuthProviders || []
}

export async function removeLinkedIdentity(identityId) {
  const data = await graphqlRequest(REMOVE_LINKED_IDENTITY_MUTATION, { identityId })
  return data.removeLinkedIdentity
}

export function oauthErrorMessage(code) {
  switch (code) {
    case 'not_configured':
      return 'That sign-in provider is not available right now.'
    case 'invalid_state':
      return 'Sign-in session expired. Please try again.'
    case 'auth_required':
      return 'Sign in first, then try linking your account again.'
    case 'merge_required':
      return 'This account belongs to another JoinQuest profile. Confirm the merge to continue.'
    case 'last_method':
      return 'Keep at least one sign-in method on your account.'
    default:
      return 'Could not sign in with that provider. Please try again.'
  }
}
