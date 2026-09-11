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
    # Did this player come into the match with the viewer — the same room table. The
    # regroup card shows the viewer's own group, not everyone the match contained (JQ-291).
    arrivalParty
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
 * The three regroup failures a client should treat differently, as the backend names them
 * in the `RegroupErrorCode` enum (backend/graph/schema/match.graphqls). The resolver puts
 * one of these in the GraphQL error's `code` extension; `graphqlRequest` hangs it on the
 * thrown Error as `.code`.
 *
 * Values are the enum's, exactly — `src/test/regroupErrorCodes.test.js` reads the schema
 * and fails if one of them stops existing. `UNKNOWN` is this module's own: it is every
 * failure the backend sends no code with, and has no enum member.
 */
export const REGROUP_ERROR = {
  NO_MODE: 'NO_REGROUP_MODE',
  NOT_FINISHED: 'SESSION_NOT_FINISHED',
  TABLE_FULL: 'TABLE_FULL',
  UNKNOWN: 'UNKNOWN',
}

const REGROUP_ERROR_CODES = new Set([
  REGROUP_ERROR.NO_MODE,
  REGROUP_ERROR.NOT_FINISHED,
  REGROUP_ERROR.TABLE_FULL,
])

/**
 * Which of the three a thrown error is, or UNKNOWN. Reads the code and nothing else: the
 * messages beside these codes are copy, and rewording one must not move a player onto a
 * different branch (JQ-176).
 */
export function classifyRegroupError(error) {
  const code = typeof error === 'object' && error !== null ? error.code : undefined
  return REGROUP_ERROR_CODES.has(code) ? code : REGROUP_ERROR.UNKNOWN
}
