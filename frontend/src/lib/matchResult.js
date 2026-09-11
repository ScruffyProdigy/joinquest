import { createClient } from 'graphql-ws'
import { getGraphQLWsUrl } from './env'
import { graphqlRequest } from './graphql'
import { prefetchSubscriptionAuth } from './queue'
import { PUBLIC_PLAYER_FIELDS } from './avatars'
import { TABLE_FIELDS } from './tables'

// PublicPlayer (not User): a queue match introduces strangers, so the roster
// must not expose contact details — no email, no avatarKey.
const MATCH_RESULT_FIELDS = `
  matchId
  status
  reported
  complete
  endedAt
  regroupInviteCode
  # Did this viewer reach the match from a table they sat at with a group, or from the
  # catalog queue on their own. The rejoin rules differ by design (JQ-232).
  groupPlay
  game {
    id
    slug
    name
    # The game's own accent, for the gradient on the primary action — the one place this
    # screen belongs to the game rather than to JoinQuest (JQ-277).
    accentColor
    modes {
      id
      status
      minPlayers
    }
  }
  # The mode actually played. Nullable: sessions outlive modes, so the screen keeps a
  # fallback for null rather than assuming it is always here.
  mode {
    id
    modeKey
    displayName
    minPlayers
    # Is there anything to choose here at all: a role select, or pre-queue options. A
    # solo player whose mode has neither gets no "choose again" action.
    hasPreMatchChoice
  }
  participants {
    user {
      ${PUBLIC_PLAYER_FIELDS}
    }
    role
    finished
    finishedAt
    placement
    winner
    reason
    regroup
  }
`

const MATCH_RESULT_QUERY = `
  query MatchResult($matchId: ID!) {
    matchResult(matchId: $matchId) {
      ${MATCH_RESULT_FIELDS}
    }
  }
`

const PLAY_AGAIN_MUTATION = `
  mutation PlayAgain($matchId: ID!) {
    playAgain(matchId: $matchId) {
      table {
        ${TABLE_FIELDS}
      }
      inviteCode
      seated
    }
  }
`

const DECLINE_PLAY_AGAIN_MUTATION = `
  mutation DeclinePlayAgain($matchId: ID!) {
    declinePlayAgain(matchId: $matchId) {
      path
      kind
    }
  }
`

const MATCH_RESULT_UPDATED_SUBSCRIPTION = `
  subscription MatchResultUpdated($matchId: ID!) {
    matchResultUpdated(matchId: $matchId) {
      ${MATCH_RESULT_FIELDS}
    }
  }
`

let wsClient = null

async function loadSubscriptionAuth() {
  return prefetchSubscriptionAuth()
}

function getSubscriptionConnectionParams() {
  return async () => {
    const header = await loadSubscriptionAuth()
    if (!header) {
      throw new Error('Sign in required for live updates')
    }
    return { Authorization: header }
  }
}

function getWsClient() {
  if (!wsClient) {
    wsClient = createClient({
      url: getGraphQLWsUrl(),
      connectionParams: getSubscriptionConnectionParams(),
      retryAttempts: 10,
      retryWait: async (retries) => Math.min(500 * retries, 5000),
      shouldRetry: () => true,
      lazy: false,
    })
  }
  return wsClient
}

function formatSubscriptionError(err) {
  if (!err) {
    return 'Live updates unavailable'
  }
  if (typeof err === 'string') {
    return err
  }
  if (Array.isArray(err)) {
    return err[0]?.message || 'Live updates unavailable'
  }
  if (err.message) {
    return err.message
  }
  return 'Live updates unavailable'
}

export async function fetchMatchResult(matchId) {
  const data = await graphqlRequest(MATCH_RESULT_QUERY, { matchId })
  return data.matchResult
}

export async function playAgain(matchId) {
  const data = await graphqlRequest(PLAY_AGAIN_MUTATION, { matchId })
  return data.playAgain
}

export async function declinePlayAgain(matchId) {
  const data = await graphqlRequest(DECLINE_PLAY_AGAIN_MUTATION, { matchId })
  return data.declinePlayAgain
}

export async function subscribeToMatchResult(matchId, { onUpdate, onError } = {}) {
  await loadSubscriptionAuth()
  const client = getWsClient()

  const unsubscribe = client.subscribe(
    {
      query: MATCH_RESULT_UPDATED_SUBSCRIPTION,
      variables: { matchId },
    },
    {
      next: (payload) => {
        if (payload?.errors?.length) {
          onError?.(formatSubscriptionError(payload.errors))
          return
        }
        if (payload?.data?.matchResultUpdated) {
          onUpdate?.(payload.data.matchResultUpdated)
        }
      },
      error: (err) => onError?.(formatSubscriptionError(err)),
      complete: () => {},
    },
  )

  return () => {
    unsubscribe()
  }
}

/**
 * The three regroup failures a client should treat differently, as the resolver phrases
 * them (backend/graph/match_helpers.go, `regroupClientError`). GraphQL carries no error
 * code on this path, so the sent message text is all there is to match on.
 */
export const REGROUP_ERROR = {
  NO_MODE: 'NO_REGROUP_MODE',
  NOT_FINISHED: 'SESSION_NOT_FINISHED',
  TABLE_FULL: 'TABLE_FULL',
  UNKNOWN: 'UNKNOWN',
}

export function classifyRegroupError(error) {
  const message = typeof error === 'string' ? error : error?.message || ''
  if (/no longer has a mode/i.test(message)) {
    return REGROUP_ERROR.NO_MODE
  }
  // The server writes a straight apostrophe; tolerate a curly one in case the copy is retouched.
  if (/hasn['\u2019]t finished yet/i.test(message)) {
    return REGROUP_ERROR.NOT_FINISHED
  }
  if (/table is full/i.test(message)) {
    return REGROUP_ERROR.TABLE_FULL
  }
  return REGROUP_ERROR.UNKNOWN
}
