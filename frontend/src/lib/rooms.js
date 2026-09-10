import { createClient } from 'graphql-ws'
import { getGraphQLWsUrl } from './env'
import { graphqlRequest } from './graphql'
import { prefetchSubscriptionAuth } from './queue'
import { TABLE_FIELDS } from './tables'
import { USER_AVATAR_FIELDS } from './avatars'

const ROOM_FIELDS = `
  id
  inviteCode
  joinUrl
  host {
    ${USER_AVATAR_FIELDS}
  }
  members {
    ${USER_AVATAR_FIELDS}
  }
`

const ROOM_WITH_TABLES = `
  ${ROOM_FIELDS}
  messages {
    id
    body
    createdAt
    author {
      ${USER_AVATAR_FIELDS}
    }
  }
  tables {
    ${TABLE_FIELDS}
  }
`

const CREATE_ROOM_MUTATION = `
  mutation CreateRoom {
    createRoom {
      ${ROOM_FIELDS}
    }
  }
`

const JOIN_ROOM_MUTATION = `
  mutation JoinRoom($inviteCode: String!) {
    joinRoom(inviteCode: $inviteCode) {
      ${ROOM_FIELDS}
    }
  }
`

const LEAVE_ROOM_MUTATION = `
  mutation LeaveRoom {
    leaveRoom
  }
`

const SEND_ROOM_MESSAGE_MUTATION = `
  mutation SendRoomMessage($roomId: ID!, $body: String!) {
    sendRoomMessage(roomId: $roomId, body: $body) {
      id
      body
      createdAt
      author {
        ${USER_AVATAR_FIELDS}
      }
    }
  }
`

const MY_ROOM_QUERY = `
  query MyRoom {
    myRoom {
      ${ROOM_WITH_TABLES}
    }
  }
`

const ROOM_QUERY = `
  query Room($inviteCode: String!) {
    room(inviteCode: $inviteCode) {
      ${ROOM_WITH_TABLES}
    }
  }
`

const TABLE_UPDATED_SUBSCRIPTION = `
  subscription TableUpdated($roomId: ID!) {
    tableUpdated(roomId: $roomId) {
      ${TABLE_FIELDS}
    }
  }
`

const ROOM_UPDATED_SUBSCRIPTION = `
  subscription RoomUpdated($roomId: ID!) {
    roomUpdated(roomId: $roomId) {
      ${ROOM_FIELDS}
    }
  }
`

const ROOM_MESSAGE_ADDED_SUBSCRIPTION = `
  subscription RoomMessageAdded($roomId: ID!) {
    roomMessageAdded(roomId: $roomId) {
      id
      body
      createdAt
      author {
        ${USER_AVATAR_FIELDS}
      }
    }
  }
`

/**
 * How long this client keeps asking to come back before it gives up, expressed as a
 * retry count because that is what graphql-ws takes.
 *
 * This number and `store.DefaultRoomDisconnectGrace`
 * (backend/internal/store/room_presence.go) are one decision with two homes, and moving
 * either alone is a bug: the server holds a member's place for exactly as long as their
 * client is still asking for it. Raise the retries without the server window and a
 * reconnect finds the room already gone; raise the server window without the retries and
 * the room is held open for a browser that stopped trying minutes ago.
 *
 * graphql-ws passes `retryWait` a 0-based count, so the budget is
 * sum(min(500 * i, 5000)) for i in 0..64 — 22.5s across the ramp, then 55 waits at the
 * 5s ceiling, for 297.5s. That lands just under the server's 5 minutes, which is the
 * right side to be on: the client is still asking at the moment the server gives up.
 */
export const ROOM_RECONNECT_ATTEMPTS = 65

/** The backoff curve itself, exported so the budget above can be verified rather than asserted. */
export const roomReconnectWaitMs = (retries) => Math.min(500 * retries, 5000)

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
      retryAttempts: ROOM_RECONNECT_ATTEMPTS,
      retryWait: async (retries) => roomReconnectWaitMs(retries),
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

export async function createRoom() {
  const data = await graphqlRequest(CREATE_ROOM_MUTATION)
  return data.createRoom
}

export async function joinRoom(inviteCode) {
  const data = await graphqlRequest(JOIN_ROOM_MUTATION, { inviteCode })
  return data.joinRoom
}

export async function leaveRoom() {
  const data = await graphqlRequest(LEAVE_ROOM_MUTATION)
  return data.leaveRoom
}

export async function sendRoomMessage(roomId, body) {
  const data = await graphqlRequest(SEND_ROOM_MESSAGE_MUTATION, { roomId, body })
  return data.sendRoomMessage
}

export async function fetchMyRoom() {
  const data = await graphqlRequest(MY_ROOM_QUERY)
  return data.myRoom
}

export async function fetchRoom(inviteCode) {
  const data = await graphqlRequest(ROOM_QUERY, { inviteCode })
  return data.room
}

export async function subscribeToRoom(roomId, { onRoomUpdate, onMessage, onTableUpdate, onError } = {}) {
  await loadSubscriptionAuth()
  const client = getWsClient()

  const unsubRoom = client.subscribe(
    {
      query: ROOM_UPDATED_SUBSCRIPTION,
      variables: { roomId },
    },
    {
      next: (payload) => {
        if (payload?.errors?.length) {
          onError?.(formatSubscriptionError(payload.errors))
          return
        }
        if (payload?.data?.roomUpdated) {
          onRoomUpdate?.(payload.data.roomUpdated)
        }
      },
      error: (err) => onError?.(formatSubscriptionError(err)),
      complete: () => {},
    },
  )

  const unsubMessages = client.subscribe(
    {
      query: ROOM_MESSAGE_ADDED_SUBSCRIPTION,
      variables: { roomId },
    },
    {
      next: (payload) => {
        if (payload?.errors?.length) {
          onError?.(formatSubscriptionError(payload.errors))
          return
        }
        if (payload?.data?.roomMessageAdded) {
          onMessage?.(payload.data.roomMessageAdded)
        }
      },
      error: (err) => onError?.(formatSubscriptionError(err)),
      complete: () => {},
    },
  )

  const unsubTables = client.subscribe(
    {
      query: TABLE_UPDATED_SUBSCRIPTION,
      variables: { roomId },
    },
    {
      next: (payload) => {
        if (payload?.errors?.length) {
          onError?.(formatSubscriptionError(payload.errors))
          return
        }
        if (payload?.data?.tableUpdated) {
          onTableUpdate?.(payload.data.tableUpdated)
        }
      },
      error: (err) => onError?.(formatSubscriptionError(err)),
      complete: () => {},
    },
  )

  return () => {
    unsubRoom()
    unsubMessages()
    unsubTables()
  }
}

export function roomShareText(joinUrl) {
  return `Join my room on JoinQuest: ${joinUrl}`
}

export function parseRoomInviteCode(pathname = window.location.pathname) {
  const match = pathname.match(/^\/room\/([A-Za-z0-9]+)\/?$/)
  return match ? match[1].toUpperCase() : null
}
