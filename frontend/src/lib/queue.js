import { createClient } from 'graphql-ws'
import { getGraphQLUrl, getGraphQLWsUrl } from './env'
import { graphqlRequest } from './graphql'
import { isLobbyDebugEnabled, lobbyDebug } from './lobbyDebug'

const JOIN_QUEUE_MUTATION = `
  mutation JoinQueue($queueId: ID!, $queuePath: String, $options: [QueueOptionSelectionInput!]) {
    joinQueue(queueId: $queueId, queuePath: $queuePath, options: $options) {
      queued
      queuedCount
      queuePath
      sessionId
      joinUrl
      message
    }
  }
`

const MY_QUEUE_STATUS_QUERY = `
  query MyQueueStatus($queueId: ID!) {
    myQueueStatus(queueId: $queueId) {
      queued
      sessionId
      joinUrl
      queuedCount
      queuePath
    }
  }
`

const SUBSCRIPTION_AUTH_QUERY = `
  query SubscriptionAuth {
    subscriptionAuth
  }
`

const LEAVE_QUEUE_MUTATION = `
  mutation LeaveQueue($queueId: ID!) {
    leaveQueue(queueId: $queueId)
  }
`

const QUEUE_UPDATED_SUBSCRIPTION = `
  subscription QueueUpdated($queueId: ID!) {
    queueUpdated(queueId: $queueId) {
      gameId
      queueId
      status
      sessionId
      joinUrl
      queuedCount
      message
    }
  }
`

let cachedSubscriptionAuth = null
let subscriptionAuthPromise = null
let wsClient = null
let queueWsConnected = false
const queueWsConnectionListeners = new Set()

function notifyQueueWsConnection(connected) {
  if (queueWsConnected === connected) {
    return
  }
  queueWsConnected = connected
  queueWsConnectionListeners.forEach((listener) => {
    listener(connected)
  })
}

/** True when the queue subscription WebSocket is connected. */
export function isQueueWsConnected() {
  return queueWsConnected
}

/** Subscribe to queue WebSocket connect/disconnect (for live-update UX). */
export function onQueueWsConnectionChange(listener) {
  queueWsConnectionListeners.add(listener)
  listener(queueWsConnected)
  return () => queueWsConnectionListeners.delete(listener)
}

async function loadSubscriptionAuth() {
  if (cachedSubscriptionAuth) {
    return cachedSubscriptionAuth
  }
  if (!subscriptionAuthPromise) {
    subscriptionAuthPromise = graphqlRequest(SUBSCRIPTION_AUTH_QUERY)
      .then((data) => {
        const header = data.subscriptionAuth?.trim() || null
        cachedSubscriptionAuth = header
        return header
      })
      .finally(() => {
        subscriptionAuthPromise = null
      })
  }
  return subscriptionAuthPromise
}

/** Ensure subscriptionAuth is loaded (call after sign-in, before subscribing). */
export async function prefetchSubscriptionAuth() {
  return loadSubscriptionAuth()
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

function wsClientLifecycleHandlers() {
  const handlers = {
    connecting: () => notifyQueueWsConnection(false),
    connected: () => notifyQueueWsConnection(true),
    closed: () => notifyQueueWsConnection(false),
    error: () => notifyQueueWsConnection(false),
  }
  if (!isLobbyDebugEnabled()) {
    return handlers
  }
  return {
    ...handlers,
    connecting: () => {
      notifyQueueWsConnection(false)
      lobbyDebug('queue:ws:connecting', { url: getGraphQLWsUrl() })
    },
    connected: () => {
      notifyQueueWsConnection(true)
      lobbyDebug('queue:ws:connected', { url: getGraphQLWsUrl() })
    },
    closed: (event) => {
      notifyQueueWsConnection(false)
      lobbyDebug('queue:ws:closed', {
        url: getGraphQLWsUrl(),
        code: event?.code,
        reason: event?.reason,
      })
    },
    error: (error) => {
      notifyQueueWsConnection(false)
      lobbyDebug('queue:ws:error', { url: getGraphQLWsUrl(), error: String(error) })
    },
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
      on: wsClientLifecycleHandlers(),
    })
  }
  return wsClient
}

/** Clear cached WS auth after logout or session refresh. */
export function clearSubscriptionAuthCache() {
  cachedSubscriptionAuth = null
  subscriptionAuthPromise = null
  notifyQueueWsConnection(false)
  if (wsClient) {
    wsClient.dispose()
    wsClient = null
  }
}

export async function joinQueue(queueId, queuePath, options) {
  const variables = { queueId }
  if (typeof queuePath === 'string' && queuePath.trim()) {
    variables.queuePath = queuePath.trim()
  }
  // Omitted rather than sent empty: a mode without a picker must not start
  // receiving an options argument it never asked for.
  if (Array.isArray(options) && options.length > 0) {
    variables.options = options
  }
  const data = await graphqlRequest(JOIN_QUEUE_MUTATION, variables)
  return data.joinQueue
}

export async function fetchMyQueueStatus(queueId) {
  const data = await graphqlRequest(MY_QUEUE_STATUS_QUERY, { queueId })
  return data.myQueueStatus
}

/**
 * Leave the queue while the document is being torn down (refresh, tab close).
 * keepalive, not sendBeacon: the endpoint may be cross-origin and sendBeacon cannot
 * preflight a JSON body. Best effort — the browser may still drop it.
 */
export function leaveQueueOnExit(queueId) {
  if (!queueId) {
    return
  }
  try {
    void fetch(getGraphQLUrl(), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      keepalive: true,
      body: JSON.stringify({ query: LEAVE_QUEUE_MUTATION, variables: { queueId } }),
    }).catch(() => {})
  } catch {
    // Nobody left to tell.
  }
}

export async function leaveQueue(queueId) {
  const data = await graphqlRequest(LEAVE_QUEUE_MUTATION, { queueId })
  return data.leaveQueue
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

export async function subscribeToQueue(queueId, { onUpdate, onError } = {}) {
  await prefetchSubscriptionAuth()

  const client = getWsClient()
  lobbyDebug('queue:subscribe:start', { queueId })

  return client.subscribe(
    {
      query: QUEUE_UPDATED_SUBSCRIPTION,
      variables: { queueId },
    },
    {
      next: (payload) => {
        if (payload?.errors?.length) {
          const message = formatSubscriptionError(payload.errors)
          lobbyDebug('queue:subscribe:graphql-error', { queueId, message })
          onError?.(message)
          return
        }
        if (payload?.data?.queueUpdated) {
          const update = payload.data.queueUpdated
          lobbyDebug('queue:subscribe:update', {
            queueId,
            status: update.status,
            hasJoinUrl: Boolean(update.joinUrl),
            queuedCount: update.queuedCount,
          })
          onUpdate?.(update)
        } else {
          lobbyDebug('queue:subscribe:empty-payload', { queueId })
        }
      },
      error: (err) => {
        const message = formatSubscriptionError(err)
        lobbyDebug('queue:subscribe:error', { queueId, message })
        onError?.(message)
      },
      complete: () => {
        lobbyDebug('queue:subscribe:complete', { queueId })
      },
    },
  )
}
