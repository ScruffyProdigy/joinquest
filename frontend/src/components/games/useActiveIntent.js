import { useCallback, useEffect, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { fetchMyActiveIntent, leaveActiveGame, resolveIntentLaunchUrl } from '../../lib/intent'
import {
  fetchMyQueueStatus,
  leaveQueue,
  onQueueWsConnectionChange,
  prefetchSubscriptionAuth,
  subscribeToQueue,
} from '../../lib/queue'
import { subscribeToMyTableSeat, TABLE_UPDATED_EVENT, fetchMyTableSeat } from '../../lib/tables'
import { lobbyDebug } from '../../lib/lobbyDebug'
import { LEAVE_GAME_FAILED, LEAVE_GAME_NOT_FOUND } from '../../lib/playerCopy'
import { onTabVisible } from '../../lib/tabVisibility'
import { useActiveTableSeat } from './useActiveTableSeat'

function mergeMatchedJoinUrl(intent, joinUrl) {
  if (!intent || intent.status !== 'MATCHED' || !joinUrl || intent.joinUrl === joinUrl) {
    return intent
  }
  return { ...intent, joinUrl }
}

export function useActiveIntent() {
  const { user, loading: authLoading } = useAuth()
  const [activeIntent, setActiveIntent] = useState(null)
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [leaveError, setLeaveError] = useState(null)
  const [queueWsConnected, setQueueWsConnected] = useState(false)
  const queueUnsubRef = useRef(null)
  const seatUnsubRef = useRef(null)
  const joinGraceUntilRef = useRef(0)

  const {
    activeTableSeat,
    loading: tableLoading,
    busy: tableBusy,
    refresh: refreshTable,
    handleLeave: leaveTableSeat,
  } = useActiveTableSeat()

  const enrichMatchedJoinUrl = useCallback(async (intent, tableSeat) => {
    if (intent?.status !== 'MATCHED' || intent?.joinUrl) {
      return intent
    }
    if (tableSeat?.joinUrl) {
      return { ...intent, joinUrl: tableSeat.joinUrl }
    }
    if (!intent?.queueId) {
      return intent
    }
    try {
      const status = await fetchMyQueueStatus(intent.queueId)
      if (status?.joinUrl) {
        return { ...intent, joinUrl: status.joinUrl }
      }
    } catch {
      // Provision may still be in flight; banner can retry on the next refresh.
    }
    return intent
  }, [])

  const refreshIntent = useCallback(async (reason = 'unknown') => {
    if (!user) {
      setActiveIntent(null)
      return
    }
    const startedAt = Date.now()
    lobbyDebug('intent:refresh:start', { reason, queueId: activeIntent?.queueId ?? null })
    setLoading(true)
    try {
      const [intentResult, tableResult] = await Promise.allSettled([
        fetchMyActiveIntent(),
        fetchMyTableSeat(),
      ])
      if (intentResult.status === 'rejected') {
        lobbyDebug('intent:refresh:failed', {
          reason,
          error: intentResult.reason?.message || String(intentResult.reason),
          ms: Date.now() - startedAt,
        })
        return
      }
      const tableSeat = tableResult.status === 'fulfilled' ? tableResult.value : null
      const next = await enrichMatchedJoinUrl(intentResult.value, tableSeat)
      if (next) {
        lobbyDebug('intent:refresh:done', {
          reason,
          status: next.status,
          queueId: next.queueId ?? null,
          hasJoinUrl: Boolean(next.joinUrl),
          ms: Date.now() - startedAt,
        })
        setActiveIntent(next)
        return
      }
      if (intentResult.value === null && Date.now() < joinGraceUntilRef.current) {
        lobbyDebug('intent:refresh:grace', { reason, ms: Date.now() - startedAt })
        return
      }
      lobbyDebug('intent:refresh:cleared', { reason, ms: Date.now() - startedAt })
      setActiveIntent(null)
    } finally {
      setLoading(false)
    }
  }, [enrichMatchedJoinUrl, user, activeIntent?.queueId])

  const refresh = useCallback(async () => {
    await Promise.all([refreshIntent(), refreshTable()])
  }, [refreshIntent, refreshTable])

  useEffect(() => {
    if (authLoading) {
      return
    }
    void refreshIntent('auth-load')
  }, [authLoading, refreshIntent])

  useEffect(() => {
    const handler = () => {
      void refresh()
    }
    window.addEventListener(TABLE_UPDATED_EVENT, handler)
    return () => window.removeEventListener(TABLE_UPDATED_EVENT, handler)
  }, [refresh])

  useEffect(() => {
    return onTabVisible(() => {
      void refresh('visibility')
    })
  }, [refresh])

  useEffect(() => {
    return onQueueWsConnectionChange(setQueueWsConnected)
  }, [])

  useEffect(() => {
    queueUnsubRef.current?.()
    queueUnsubRef.current = null

    const queueId = activeIntent?.queueId
    if (!user || !queueId) {
      return undefined
    }

    let cancelled = false
    lobbyDebug('intent:queue:subscribe-effect', { queueId, status: activeIntent?.status ?? null })

    void (async () => {
      try {
        await prefetchSubscriptionAuth()
        if (cancelled) {
          return
        }
        const unsubscribe = await subscribeToQueue(queueId, {
          onUpdate: (update) => {
            lobbyDebug('intent:queue:ws-update', {
              queueId,
              status: update?.status,
              hasJoinUrl: Boolean(update?.joinUrl),
            })
            if (update?.status === 'MATCHED') {
              setActiveIntent((prev) => {
                if (!prev) {
                  lobbyDebug('intent:queue:ws-update-skipped', { queueId, reason: 'no-prev-intent' })
                  return prev
                }
                return {
                  ...prev,
                  status: 'MATCHED',
                  joinUrl: update.joinUrl ?? prev.joinUrl ?? null,
                }
              })
            } else if (update?.status === 'WAITING') {
              setActiveIntent((prev) => {
                if (!prev) {
                  lobbyDebug('intent:queue:ws-update-skipped', { queueId, reason: 'no-prev-intent' })
                  return prev
                }
                return {
                  ...prev,
                  status: 'WAITING',
                  queuedCount: update.queuedCount ?? prev.queuedCount,
                }
              })
            }
            void refreshIntent('ws-followup')
          },
          onError: (message) => {
            lobbyDebug('intent:queue:ws-error', { queueId, message })
          },
        })
        if (cancelled) {
          unsubscribe()
          return
        }
        queueUnsubRef.current = unsubscribe
        lobbyDebug('intent:queue:subscribed', { queueId })
      } catch (err) {
        lobbyDebug('intent:queue:subscribe-failed', {
          queueId,
          error: err?.message || String(err),
        })
      }
    })()

    return () => {
      cancelled = true
      queueUnsubRef.current?.()
      queueUnsubRef.current = null
    }
    // activeIntent?.status is read only to label the debug log above. Listing it
    // would tear down and re-open the queue subscription on every WAITING ->
    // MATCHED transition, so it stays out of the deps on purpose.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user, activeIntent?.queueId, refreshIntent])

  useEffect(() => {
    if (activeIntent?.status !== 'WAITING') {
      return undefined
    }
    const timer = window.setInterval(() => {
      void refreshIntent('poll-waiting')
    }, 2000)
    return () => window.clearInterval(timer)
  }, [activeIntent?.status, activeIntent?.queueId, refreshIntent])

  useEffect(() => {
    seatUnsubRef.current?.()
    seatUnsubRef.current = null

    if (authLoading || !user) {
      return undefined
    }

    let cancelled = false

    void (async () => {
      try {
        await prefetchSubscriptionAuth()
        if (cancelled) {
          return
        }
        const unsubscribe = await subscribeToMyTableSeat({
          onUpdate: (seat) => {
            if (seat?.joinUrl) {
              setActiveIntent((prev) => mergeMatchedJoinUrl(prev, seat.joinUrl))
            }
            void refresh()
          },
        })
        if (cancelled) {
          unsubscribe()
          return
        }
        seatUnsubRef.current = unsubscribe
      } catch {
        // Banner still works via refresh.
      }
    })()

    return () => {
      cancelled = true
      seatUnsubRef.current?.()
      seatUnsubRef.current = null
    }
  }, [authLoading, user, refresh])

  useEffect(() => {
    if (activeIntent?.status !== 'MATCHED' || resolveIntentLaunchUrl(activeIntent, activeTableSeat)) {
      return undefined
    }
    const timer = window.setInterval(() => {
      void refreshIntent('poll-matched-join-url')
    }, 3000)
    return () => window.clearInterval(timer)
  }, [activeIntent, activeTableSeat, refreshIntent])

  const leaveQueueIntent = useCallback(async () => {
    if (!activeIntent?.queueId) {
      return
    }
    joinGraceUntilRef.current = 0
    setBusy(true)
    try {
      await leaveQueue(activeIntent.queueId)
      await refreshIntent('leave-queue')
    } finally {
      setBusy(false)
    }
  }, [activeIntent?.queueId, refreshIntent])

  const handleLeave = useCallback(async () => {
    setLeaveError(null)
    if (activeIntent?.status === 'MATCHED' || activeTableSeat?.status === 'started') {
      setBusy(true)
      try {
        const left = await leaveActiveGame()
        await refresh()
        // The server tells us whether it actually unwound anything. When it did
        // not, the banner we just acted on was stale: the refresh normally clears
        // it, and this message only stays on screen if the player is still stuck.
        if (!left) {
          lobbyDebug('intent:leave:nothing-to-leave', { queueId: activeIntent?.queueId ?? null })
          setLeaveError(LEAVE_GAME_NOT_FOUND)
        }
      } catch (err) {
        lobbyDebug('intent:leave:failed', { error: err?.message || String(err) })
        setLeaveError(LEAVE_GAME_FAILED)
      } finally {
        setBusy(false)
      }
      return
    }
    try {
      if (activeIntent?.status === 'WAITING') {
        await leaveQueueIntent()
        return
      }
      await leaveTableSeat()
    } catch (err) {
      lobbyDebug('intent:leave:failed', { error: err?.message || String(err) })
      setLeaveError(LEAVE_GAME_FAILED)
    }
  }, [
    activeIntent?.queueId,
    activeIntent?.status,
    activeTableSeat?.status,
    leaveQueueIntent,
    leaveTableSeat,
    refresh,
  ])

  const notifyQueueJoined = useCallback((queueId, result, { gameId, gameName, modeName, queuePathDisplayName } = {}) => {
    if (!result) {
      return
    }
    joinGraceUntilRef.current = Date.now() + 3000
    lobbyDebug('intent:queue-joined', {
      queueId,
      queued: Boolean(result.queued),
      sessionId: result.sessionId ?? null,
    })
    if (result.queued) {
      setActiveIntent({
        queueId,
        gameId,
        gameName,
        modeName,
        status: 'WAITING',
        queuedCount: result.queuedCount ?? 1,
        queuePath: result.queuePath ?? null,
        queuePathDisplayName: queuePathDisplayName ?? null,
        formingGaps: [],
      })
      return
    }
    if (result.sessionId) {
      setActiveIntent({
        queueId,
        gameId,
        gameName,
        modeName,
        status: 'MATCHED',
        joinUrl: result.joinUrl ?? null,
      })
    }
  }, [])

  return {
    activeIntent,
    activeTableSeat,
    loading: loading || tableLoading,
    busy: busy || tableBusy,
    leaveError,
    queueWsConnected,
    refresh,
    notifyQueueJoined,
    handleLeave,
  }
}
