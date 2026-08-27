import { graphqlRequest } from './graphql'
import { clearSubscriptionAuthCache } from './queue'

const USER_FIELDS = `
  id
  email
  displayName
  avatarUrl
  avatarKey
  avatarSource
  createdAt
  isGuest
`

const ME_QUERY = `
  query Me {
    me {
      ${USER_FIELDS}
    }
  }
`

const PREVIEW_LINK_EMAIL_QUERY = `
  query PreviewLinkEmail($email: String!) {
    previewLinkEmail(email: $email) {
      email
      willMergeAccounts
      mergeSourceDisplayName
    }
  }
`

const MY_ACCOUNT_QUERY = `
  query MyAccount {
    myAccount {
      signInMethodCount
      emails {
        id
        email
        isPrimary
        verifiedAt
      }
      identities {
        id
        provider
        email
        verifiedAt
      }
      user {
        ${USER_FIELDS}
      }
    }
  }
`

const REQUEST_SIGN_IN_MUTATION = `
  mutation RequestSignIn($email: String!) {
    requestSignIn(email: $email)
  }
`

const CREATE_GUEST_SESSION_MUTATION = `
  mutation CreateGuestSession {
    createGuestSession {
      ${USER_FIELDS}
    }
  }
`

const COMPLETE_SIGN_IN_WITH_LINK_MUTATION = `
  mutation CompleteSignInWithLink($token: ID!) {
    completeSignInWithLink(token: $token) {
      ${USER_FIELDS}
    }
  }
`

const COMPLETE_SIGN_IN_WITH_CODE_MUTATION = `
  mutation CompleteSignInWithCode($email: String!, $code: String!) {
    completeSignInWithCode(email: $email, code: $code) {
      ${USER_FIELDS}
    }
  }
`

const REQUEST_LINK_EMAIL_MUTATION = `
  mutation RequestLinkEmail($email: String!) {
    requestLinkEmail(email: $email)
  }
`

const COMPLETE_LINK_EMAIL_WITH_CODE_MUTATION = `
  mutation CompleteLinkEmailWithCode($email: String!, $code: String!, $confirmMerge: Boolean) {
    completeLinkEmailWithCode(email: $email, code: $code, confirmMerge: $confirmMerge) {
      ${USER_FIELDS}
    }
  }
`

const COMPLETE_LINK_EMAIL_WITH_LINK_MUTATION = `
  mutation CompleteLinkEmailWithLink($token: ID!, $confirmMerge: Boolean) {
    completeLinkEmailWithLink(token: $token, confirmMerge: $confirmMerge) {
      ${USER_FIELDS}
    }
  }
`

const REMOVE_LINKED_EMAIL_MUTATION = `
  mutation RemoveLinkedEmail($emailId: ID!) {
    removeLinkedEmail(emailId: $emailId) {
      ${USER_FIELDS}
    }
  }
`

const SET_PRIMARY_EMAIL_MUTATION = `
  mutation SetPrimaryEmail($emailId: ID!) {
    setPrimaryEmail(emailId: $emailId) {
      ${USER_FIELDS}
    }
  }
`

const LOGOUT_MUTATION = `
  mutation Logout {
    logout
  }
`

export async function fetchCurrentUser() {
  const data = await graphqlRequest(ME_QUERY)
  return data.me
}

export async function fetchMyAccount() {
  const data = await graphqlRequest(MY_ACCOUNT_QUERY)
  return data.myAccount
}

export async function createGuestSession() {
  const data = await graphqlRequest(CREATE_GUEST_SESSION_MUTATION)
  clearSubscriptionAuthCache()
  return data.createGuestSession
}

export async function requestSignIn(email) {
  const data = await graphqlRequest(REQUEST_SIGN_IN_MUTATION, { email })
  return data.requestSignIn === true
}

export async function completeSignInWithCode(email, code) {
  const data = await graphqlRequest(COMPLETE_SIGN_IN_WITH_CODE_MUTATION, { email, code })
  clearSubscriptionAuthCache()
  return data.completeSignInWithCode
}

export async function logout() {
  const data = await graphqlRequest(LOGOUT_MUTATION)
  clearSubscriptionAuthCache()
  return data.logout === true
}

export async function completeSignInWithLink(token) {
  const data = await graphqlRequest(COMPLETE_SIGN_IN_WITH_LINK_MUTATION, { token })
  clearSubscriptionAuthCache()
  return data.completeSignInWithLink
}

export const MERGE_CONFIRMATION_MESSAGE =
  'This email belongs to another JoinQuest account. Confirm the merge to continue.'

export function isMergeConfirmationRequired(message) {
  return String(message || '').includes('Confirm the merge')
}

export async function previewLinkEmail(email) {
  const data = await graphqlRequest(PREVIEW_LINK_EMAIL_QUERY, { email })
  return data.previewLinkEmail
}

export async function requestLinkEmail(email) {
  const data = await graphqlRequest(REQUEST_LINK_EMAIL_MUTATION, { email })
  return data.requestLinkEmail === true
}

export async function completeLinkEmailWithCode(email, code, confirmMerge = false) {
  const data = await graphqlRequest(COMPLETE_LINK_EMAIL_WITH_CODE_MUTATION, {
    email,
    code,
    confirmMerge,
  })
  clearSubscriptionAuthCache()
  return data.completeLinkEmailWithCode
}

export async function completeLinkEmailWithLink(token, confirmMerge = false) {
  const data = await graphqlRequest(COMPLETE_LINK_EMAIL_WITH_LINK_MUTATION, { token, confirmMerge })
  clearSubscriptionAuthCache()
  return data.completeLinkEmailWithLink
}

export async function removeLinkedEmail(emailId) {
  const data = await graphqlRequest(REMOVE_LINKED_EMAIL_MUTATION, { emailId })
  return data.removeLinkedEmail
}

export async function setPrimaryEmail(emailId) {
  const data = await graphqlRequest(SET_PRIMARY_EMAIL_MUTATION, { emailId })
  return data.setPrimaryEmail
}

const pendingCompletions = new Map()

export async function completeSignInWithLinkOnce(token) {
  const key = token.trim()
  if (!key) {
    throw new Error('Missing sign-in token')
  }

  if (pendingCompletions.has(key)) {
    return pendingCompletions.get(key)
  }

  const promise = completeSignInWithLink(key).catch((error) => {
    pendingCompletions.delete(key)
    throw error
  })
  pendingCompletions.set(key, promise)
  return promise
}

export async function completeLinkEmailWithLinkOnce(token, confirmMerge = false) {
  const key = `link:${token.trim()}:${confirmMerge ? '1' : '0'}`
  if (!token.trim()) {
    throw new Error('Missing verification token')
  }

  if (pendingCompletions.has(key)) {
    return pendingCompletions.get(key)
  }

  const promise = completeLinkEmailWithLink(token.trim(), confirmMerge).catch((error) => {
    pendingCompletions.delete(key)
    throw error
  })
  pendingCompletions.set(key, promise)
  return promise
}

export async function completeAuthLinkOnce(token) {
  return completeSignInWithLinkOnce(token)
}
