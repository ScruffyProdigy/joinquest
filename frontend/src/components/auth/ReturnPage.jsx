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
  RESULTS_LEAVE_MATCH,
  RESULTS_LIVE_UPDATES_OFF,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'
import { Link } from '../ui/link'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import MatchStandings from '../match/MatchStandings'
import RegroupCard from '../match/RegroupCard'
import StillPlaying from '../match/StillPlaying'
import YourResult from '../match/YourResult'
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
 * The mode the match was actually played in is authoritative — `MatchResult.mode` now
 * carries it. The scan below is only the fallback for a null mode: `game_sessions.mode_id`
 * is nullable and sessions outlive their modes, so a played mode can genuinely be gone.
 * In that case the smallest active minimum is the most permissive floor that is still a
 * real requirement, and with no mode data at all, 2 — a regroup is by definition a group.
 *
 * This number is displayed, not enforced: nothing on this screen is gated on it.
 */
function regroupMinPlayers(result) {
  const played = result?.mode?.minPlayers
  if (Number.isFinite(played) && played > 0) {
    return played
  }
  const modes = Array.isArray(result?.game?.modes) ? result.game.modes : []
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
  const [liveUpdatesOff, setLiveUpdatesOff] = useState(false)
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
              setLiveUpdatesOff(false)
              setResult(next)
            }
          },
          onError: () => {
            if (mountedRef.current) {
              setLiveUpdatesOff(true)
            }
          },
        })
        if (!mountedRef.current) {
          stop()
          return
        }
        unsubscribe = stop
      } catch {
        // The fetched result still stands, so this is a notice and not an error state —
        // but it must not stay silent: on a still-running match, live updates are the only
        // thing that would ever move this screen off "Still playing".
        if (mountedRef.current) {
          setLiveUpdatesOff(true)
        }
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
  const viewerRegroup =
    (result?.participants ?? []).find((participant) => participant.user?.id === viewerId)?.regroup ?? null

  const handlePlayAgain = useCallback(async () => {
    setRegroupError('')
    setBusy(true)
    // Already IN: the seat is claimed and the table exists, so there is nothing to opt into
    // — go straight to it rather than asking the server to claim a seat twice.
    if (viewerRegroup === 'IN' && result?.regroupInviteCode) {
      leaveTo(`/room/${result.regroupInviteCode}`)
      return
    }
    try {
      const outcome = await playAgain(matchIdRef.current)
      leaveTo(`/room/${outcome.inviteCode}`)
    } catch (error) {
      const kind = classifyRegroupError(error)
      if (kind === REGROUP_ERROR.NO_MODE) {
        // Documented degradation: the session has no mode to rebuild a table from, so send
        // the player to the game rather than showing them a dead end.
        if (!mountedRef.current) return
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
  }, [gameDetailPath, leaveTo, result?.regroupInviteCode, viewerRegroup])

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
      if (!mountedRef.current) return
      // The navigation below is a full page load, so clearing `busy` matters only if that
      // load never happens — but that is exactly the case where a stuck-disabled card would
      // strand the player, so clear it rather than relying on leaving.
      setBusy(false)
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
          {liveUpdatesOff ? (
            <p className="status-message" role="status">
              {RESULTS_LIVE_UPDATES_OFF}
            </p>
          ) : null}
          {complete ? (
            <>
              <MatchStandings result={result} viewerId={viewerId} />
              <RegroupCard
                result={result}
                viewerId={viewerId}
                minPlayers={regroupMinPlayers(result)}
                busy={busy}
                onPlayAgain={handlePlayAgain}
                onDecline={() => declineThenLeave(null)}
                onBackToGame={() => declineThenLeave(gameDetailPath)}
                // Same destination as "Back to <game>", and only ever offered in its
                // place: the mode's picker lives on the game page, so re-opening the
                // choices means going there rather than duplicating the sheet here.
                // Declining first is right — this player is not taking the fast path
                // back to this table, and the others should see that (JQ-232).
                onChooseAgain={() => declineThenLeave(gameDetailPath)}
              />
              {regroupError ? (
                <p className="status-message status-message-error" role="alert">
                  {regroupError}
                </p>
              ) : null}
            </>
          ) : (
            <>
              {/*
                The viewer's own result comes first: StillPlaying deliberately lists only the
                others, so without this the screen answers "how is everyone else doing?" and
                never "how did I do?".
              */}
              <YourResult result={result} viewerId={viewerId} />
              <StillPlaying participants={others} />
              {/*
                The only exit from this branch. Without it a player who returns mid-match
                can leave only with the browser's back button — and if the subscription
                never connects, never at all. No decline: the match has not finished, so
                there is no regroup to answer yet.
              */}
              <Button type="button" variant="outline" onClick={() => leaveTo(destinationRef.current)}>
                {RESULTS_LEAVE_MATCH}
              </Button>
            </>
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
            <Link className="self-start" href="/">
              Continue to JoinQuest
            </Link>
          ) : null}
        </CardContent>
      </Card>
    </main>
  )
}
