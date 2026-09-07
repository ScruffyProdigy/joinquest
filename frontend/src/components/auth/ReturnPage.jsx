import { useCallback, useEffect, useRef, useState } from 'react'
import { fetchReturnDestination } from '../../lib/return'
import {
  classifyRegroupError,
  declinePlayAgain,
  fetchMatchResult,
  playAgain,
  REGROUP_ERROR,
  subscribeToMatchResult,
} from '../../lib/matchResult'
import { APP_NAME } from '../../lib/brand'
import { gamePagePath } from '../../lib/gameCard'
import {
  REGROUP_ERROR_GENERIC,
  REGROUP_ERROR_NOT_FINISHED,
  REGROUP_ERROR_TABLE_FULL,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import MatchStandings from '../match/MatchStandings'
import RegroupCard from '../match/RegroupCard'
import StillPlaying from '../match/StillPlaying'
import { useAuth } from './AuthProvider'

function matchIdFromLocation() {
  const params = new URLSearchParams(window.location.search)
  return params.get('match')?.trim() || ''
}

/** Only same-origin absolute paths — a returned destination is never allowed off-site. */
function safeReturnPath(path) {
  const trimmed = path?.trim() || '/'
  return trimmed.startsWith('/') && !trimmed.startsWith('//') ? trimmed : '/'
}

/**
 * A regroup rebuilds a table for the finished session's mode, but MatchResult does not say
 * which mode was played — so the gate uses the smallest minimum across the game's active
 * modes, the most permissive floor that is still a real requirement. With no mode data at
 * all, fall back to 2: a regroup is by definition a group.
 */
function regroupMinPlayers(game) {
  const modes = Array.isArray(game?.modes) ? game.modes : []
  const active = modes.filter((mode) => mode?.status === 'active')
  const pool = active.length > 0 ? active : modes
  const mins = pool.map((mode) => mode?.minPlayers).filter((n) => Number.isFinite(n) && n > 0)
  return mins.length > 0 ? Math.min(...mins) : 2
}

export default function ReturnPage() {
  const { user } = useAuth()
  const viewerId = user?.id ?? null

  const [status, setStatus] = useState('loading')
  const [message, setMessage] = useState('Taking you back…')
  const [result, setResult] = useState(null)
  const [regroupError, setRegroupError] = useState('')
  const [busy, setBusy] = useState(false)

  const matchIdRef = useRef('')
  const destinationRef = useRef('/')
  const mountedRef = useRef(true)

  const leaveTo = useCallback((path) => {
    const safePath = safeReturnPath(path)
    window.history.replaceState({}, '', safePath)
    window.location.assign(safePath)
  }, [])

  useEffect(() => {
    mountedRef.current = true
    let unsubscribe = null
    const matchId = matchIdFromLocation()
    matchIdRef.current = matchId

    async function run() {
      // Always first, and regardless of which branch follows: this is what releases the
      // player's matched queue row (AcknowledgePlayerReturn) so they can queue again.
      let destination = '/'
      try {
        const dest = await fetchReturnDestination(matchId || null)
        destination = safeReturnPath(dest?.path)
      } catch (error) {
        if (!mountedRef.current) return
        setStatus('error')
        setMessage(error.message || 'Could not resolve return destination')
        return
      }
      if (!mountedRef.current) return
      destinationRef.current = destination

      // Branch 1: nothing to show — today's redirect, unchanged.
      if (!matchId) {
        redirect(destination)
        return
      }

      let matchResult = null
      try {
        matchResult = await fetchMatchResult(matchId)
      } catch {
        // A lookup failure (including "you did not play in this match") is not an error the
        // player should see — it just means there is no result screen for this return.
        matchResult = null
      }
      if (!mountedRef.current) return
      if (!matchResult) {
        redirect(destination)
        return
      }

      setResult(matchResult)
      setStatus('result')

      try {
        const stop = await subscribeToMatchResult(matchId, {
          onUpdate: (next) => {
            if (mountedRef.current && next) {
              setResult(next)
            }
          },
          onError: () => {},
        })
        if (!mountedRef.current) {
          stop()
          return
        }
        unsubscribe = stop
      } catch {
        // Live updates are a bonus; the fetched result already stands.
      }
    }

    function redirect(path) {
      setStatus('success')
      setMessage('Redirecting…')
      leaveTo(path)
    }

    run()

    return () => {
      mountedRef.current = false
      unsubscribe?.()
    }
  }, [leaveTo])

  const gameDetailPath = gamePagePath(result?.game) || destinationRef.current

  const handlePlayAgain = useCallback(async () => {
    setRegroupError('')
    setBusy(true)
    try {
      const outcome = await playAgain(matchIdRef.current)
      leaveTo(`/room/${outcome.inviteCode}`)
    } catch (error) {
      const kind = classifyRegroupError(error)
      if (kind === REGROUP_ERROR.NO_MODE) {
        // Documented degradation: the session has no mode to rebuild a table from, so send
        // the player to the game rather than showing them a dead end.
        leaveTo(gameDetailPath)
        return
      }
      if (!mountedRef.current) return
      setBusy(false)
      if (kind === REGROUP_ERROR.NOT_FINISHED) {
        setRegroupError(REGROUP_ERROR_NOT_FINISHED)
      } else if (kind === REGROUP_ERROR.TABLE_FULL) {
        setRegroupError(REGROUP_ERROR_TABLE_FULL)
      } else {
        setRegroupError(REGROUP_ERROR_GENERIC)
      }
    }
  }, [gameDetailPath, leaveTo])

  // Leaving the regroup is a decision the server needs, so the seat frees up for backfill —
  // but a failed decline must never trap the player on this screen.
  const declineThenLeave = useCallback(
    async (fallbackPath) => {
      setRegroupError('')
      setBusy(true)
      let path = fallbackPath
      try {
        const dest = await declinePlayAgain(matchIdRef.current)
        path = fallbackPath ?? safeReturnPath(dest?.path)
      } catch {
        path = fallbackPath ?? destinationRef.current
      }
      leaveTo(path ?? destinationRef.current)
    },
    [leaveTo],
  )

  if (status === 'result' && result) {
    const complete = Boolean(result.complete)
    const others = (result.participants ?? []).filter((participant) => participant.user?.id !== viewerId)

    return (
      <main className="flex min-h-screen flex-col items-center gap-6 bg-background p-6 text-foreground">
        <h1 className="font-heading text-2xl font-bold">{APP_NAME}</h1>
        <div className="flex w-full max-w-md flex-col gap-4">
          {complete ? (
            <>
              <MatchStandings result={result} viewerId={viewerId} />
              <RegroupCard
                result={result}
                viewerId={viewerId}
                minPlayers={regroupMinPlayers(result.game)}
                busy={busy}
                onPlayAgain={handlePlayAgain}
                onDecline={() => declineThenLeave(null)}
                onBackToGame={() => declineThenLeave(gameDetailPath)}
              />
              {regroupError ? (
                <p className="status-message status-message-error" role="alert">
                  {regroupError}
                </p>
              ) : null}
            </>
          ) : (
            <StillPlaying participants={others} />
          )}
        </div>
      </main>
    )
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-background p-6 text-foreground">
      <h1 className="font-heading text-2xl font-bold">{APP_NAME}</h1>
      <Card className="w-full max-w-sm" aria-live="polite">
        <CardHeader>
          <CardTitle as="h2" className="font-heading text-xl font-semibold">
            Welcome back
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className={status === 'error' ? 'status-message status-message-error' : 'status-message'}>
            {message}
          </p>
          {status === 'error' ? (
            <Button variant="link" className="self-start px-0" asChild>
              <a href="/">Continue to JoinQuest</a>
            </Button>
          ) : null}
        </CardContent>
      </Card>
    </main>
  )
}
