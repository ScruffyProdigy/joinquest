import { useCallback, useEffect, useRef, useState } from 'react'
import { isIdentityRequiredError } from '../../lib/graphql'
import {
  fetchMyQueueStatus,
  joinQueue,
  leaveQueue,
  prefetchSubscriptionAuth,
  subscribeToQueue,
} from '../../lib/queue'

/** Apply join mutation result (waiting or matched for the player who triggered the match). */
function applyJoinResponse(result, handlers) {
  if (result?.message) {
    handlers.setNotice?.(result.message)
  }
  syncSelectedQueuePath(result, handlers.setSelectedQueuePath)
  if (result?.joinUrl) {
    return applyQueueUpdate(
      { status: 'MATCHED', joinUrl: result.joinUrl, queuedCount: 0 },
      handlers,
    )
  }
  if (result?.sessionId) {
    applyQueueUpdate({ status: 'MATCHED', queuedCount: 0 }, handlers)
    return true
  }
  if (result?.queued) {
    return applyQueueUpdate(
      { status: 'WAITING', queuedCount: result.queuedCount ?? 1 },
      handlers,
    )
  }
  return false
}

function applyQueueUpdate(update, { setQueueState, setJoinUrl, setQueuedCount, setError, setSelectedQueuePath }) {
  if (update.status === 'MATCHED') {
    setQueueState('matched')
    if (update.joinUrl) {
      setJoinUrl(update.joinUrl)
    }
    setError('')
    return Boolean(update.joinUrl)
  }
  if (update.status === 'WAITING') {
    setQueueState('waiting')
    setQueuedCount(update.queuedCount ?? 0)
    setError('')
    return false
  }
  if (update.status === 'LEFT') {
    setQueueState('idle')
    setJoinUrl('')
    setQueuedCount(0)
    setSelectedQueuePath?.('')
    if (update.message) {
      setError(update.message)
    } else {
      setError('')
    }
    return false
  }
  return false
}

/** Map myQueueStatus query result into local queue UI state. */
function applyMyQueueStatus(result, handlers) {
  if (!result?.queued) {
    syncSelectedQueuePath(result, handlers.setSelectedQueuePath)
    if (result?.joinUrl) {
      return applyQueueUpdate(
        { status: 'MATCHED', joinUrl: result.joinUrl, queuedCount: 0 },
        handlers,
      )
    }
    applyQueueUpdate({ status: 'LEFT', queuedCount: 0 }, handlers)
    return false
  }
  syncSelectedQueuePath(result, handlers.setSelectedQueuePath)
  if (result.joinUrl) {
    return applyQueueUpdate(
      { status: 'MATCHED', joinUrl: result.joinUrl, queuedCount: result.queuedCount ?? 0 },
      handlers,
    )
  }
  return applyQueueUpdate(
    { status: 'WAITING', queuedCount: result.queuedCount ?? 0 },
    handlers,
  )
}

function syncSelectedQueuePath(result, setSelectedQueuePath) {
  if (!setSelectedQueuePath) {
    return
  }
  if (result?.queuePath) {
    setSelectedQueuePath(result.queuePath)
  } else if (!result?.queued) {
    setSelectedQueuePath('')
  }
}

/**
 * @param {string | undefined} queueId
 * @param {{ skipSubscription?: boolean }} [options] — set when the sticky banner already subscribes to this queue
 */
export function useGameQueue(queueId, { skipSubscription = false } = {}) {
  const [queueState, setQueueState] = useState('idle')
  const [joinUrl, setJoinUrl] = useState('')
  const [queuedCount, setQueuedCount] = useState(0)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [selectedQueuePath, setSelectedQueuePath] = useState('')
  const unsubscribeRef = useRef(null)
  const subscribeGenerationRef = useRef(0)
  const syncGenerationRef = useRef(0)

  const clearSubscription = useCallback(() => {
    unsubscribeRef.current?.()
    unsubscribeRef.current = null
  }, [])

  const applyUpdate = useCallback(
    (update) =>
      applyQueueUpdate(update, {
        setQueueState,
        setJoinUrl,
        setQueuedCount,
        setError,
        setSelectedQueuePath,
      }),
    [],
  )

  const ensureSubscribed = useCallback(async () => {
    if (!queueId || unsubscribeRef.current) {
      return
    }

    const generation = ++subscribeGenerationRef.current

    try {
      await prefetchSubscriptionAuth()
    } catch (err) {
      if (generation === subscribeGenerationRef.current) {
        setError(err.message || 'Sign in required for live updates')
      }
      return
    }

    if (generation !== subscribeGenerationRef.current) {
      return
    }

    try {
      const unsubscribe = await subscribeToQueue(queueId, {
        onUpdate: (update) => {
          applyUpdate(update)
        },
        onError: (message) => {
          if (generation === subscribeGenerationRef.current) {
            setError(message)
          }
        },
      })
      if (generation === subscribeGenerationRef.current) {
        unsubscribeRef.current = unsubscribe
      } else {
        unsubscribe()
      }
    } catch (err) {
      if (generation === subscribeGenerationRef.current) {
        setError(err.message || 'Live updates unavailable')
      }
    }
  }, [queueId, applyUpdate])

  useEffect(() => {
    if (!queueId) {
      return undefined
    }

    // When the intent banner owns this queue, avoid a status sync that can race
    // with the join mutation and reset local state back to idle.
    if (skipSubscription) {
      return undefined
    }

    let cancelled = false
    const syncGen = ++syncGenerationRef.current

    async function syncFromServer() {
      try {
        await prefetchSubscriptionAuth()
        if (cancelled || syncGen !== syncGenerationRef.current) {
          return
        }
        if (!skipSubscription) {
          await ensureSubscribed()
          if (cancelled || syncGen !== syncGenerationRef.current) {
            return
          }
        }
        const status = await fetchMyQueueStatus(queueId)
        if (cancelled || syncGen !== syncGenerationRef.current) {
          return
        }
        applyMyQueueStatus(status, {
          setQueueState,
          setJoinUrl,
          setQueuedCount,
          setError,
          setSelectedQueuePath,
        })
      } catch {
        // Signed-out or transient errors: keep idle until user joins.
      }
    }

    void syncFromServer()

    const onVisible = () => {
      if (document.visibilityState === 'visible') {
        const visGen = syncGenerationRef.current
        void fetchMyQueueStatus(queueId)
          .then((result) => {
            if (cancelled || visGen !== syncGenerationRef.current) {
              return
            }
            applyMyQueueStatus(result, {
              setQueueState,
              setJoinUrl,
              setQueuedCount,
              setError,
              setSelectedQueuePath,
            })
          })
          .catch(() => {})
      }
    }
    document.addEventListener('visibilitychange', onVisible)

    return () => {
      cancelled = true
      subscribeGenerationRef.current += 1
      document.removeEventListener('visibilitychange', onVisible)
      clearSubscription()
    }
  }, [queueId, skipSubscription, ensureSubscribed, clearSubscription])

  async function handleJoin(queuePath) {
    if (!queueId) {
      setError('This game is not available for group matchmaking yet')
      return null
    }
    setBusy(true)
    setError('')
    setNotice('')
    setSelectedQueuePath(typeof queuePath === 'string' ? queuePath : '')
    syncGenerationRef.current += 1
    try {
      if (!skipSubscription) {
        await ensureSubscribed()
      }
      const result = await joinQueue(queueId, queuePath)
      applyJoinResponse(result, {
        setQueueState,
        setJoinUrl,
        setQueuedCount,
        setError,
        setNotice,
        setSelectedQueuePath,
      })
      return result
    } catch (err) {
      // A missing name or avatar is answered by the identity prompt, which then
      // replays this join. Printing it here would be the bare error the prompt
      // exists to replace, so it goes back to the caller untouched.
      if (isIdentityRequiredError(err?.message)) {
        throw err
      }
      setError(err.message || 'Could not start looking for a group')
      return null
    } finally {
      setBusy(false)
    }
  }

  async function handleLeave() {
    if (!queueId) {
      return
    }
    setBusy(true)
    setError('')
    syncGenerationRef.current += 1
    try {
      await leaveQueue(queueId)
      setQueueState('idle')
      setJoinUrl('')
      setQueuedCount(0)
      setSelectedQueuePath('')
    } catch (err) {
      setError(err.message || 'Could not stop looking')
    } finally {
      setBusy(false)
    }
  }

  return {
    queueState,
    joinUrl,
    queuedCount,
    error,
    notice,
    busy,
    selectedQueuePath,
    handleJoin,
    handleLeave,
  }
}
