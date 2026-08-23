import { useCallback, useEffect, useRef, useState } from 'react'
import {
  beginSpiritAnimalReading,
  fetchMySpiritAnimalReadingWithRetry,
  fetchSpiritAnimalJourneyEligibility,
  formatPhaseCountdown,
  formatSpiritAnimalJourneyCooldown,
  friendlySpiritAnimalError,
  isSpiritAnimalAuthError,
  isSpiritAnimalFailed,
  isSpiritAnimalProcessing,
  pollSpiritAnimalReading,
  regenerateSpiritAnimalImages,
  secondsRemainingFromPhase,
  selectSpiritAnimalTotem,
  submitSpiritAnimalAnswers,
} from '../../lib/spiritAnimal'
import { slotPromptForKey } from '../../lib/spiritAnimalSlots'
import SpiritAnimalSlotGuide from './SpiritAnimalSlotGuide'
import { useAuth } from '../auth/AuthProvider'
import { Button } from '../ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import { cn } from '../../lib/utils'

const QUESTION_PANEL_CLOSE_MS = 700
const QUESTION_PANEL_PAUSE_MS = 400
const QUESTION_PANEL_OPEN_MS = 650

export default function SpiritAnimalFlow({ onComplete, onCancel }) {
  const { acceptSessionUser, clearSession } = useAuth()
  const [reading, setReading] = useState(null)
  const [questionIndex, setQuestionIndex] = useState(0)
  const [answers, setAnswers] = useState([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [phase, setPhase] = useState('intro')
  /** 'questions' = waiting for tarot Q&A; 'mascots' = generating companions after submit */
  const [processingPurpose, setProcessingPurpose] = useState(null)
  const [regenRequested, setRegenRequested] = useState(false)
  /** 'open' | 'closing' | 'closed' | 'opening' — question panel transition between answers */
  const [panelState, setPanelState] = useState('open')
  const [countdownSeconds, setCountdownSeconds] = useState(null)
  const [journeyEligibility, setJourneyEligibility] = useState(null)
  const transitionTimersRef = useRef([])
  /** Locked at processing start so poll updates do not reset the countdown. */
  const phaseTimingRef = useRef(null)
  const journeyAnchorRef = useRef(null)

  const handleFlowError = useCallback((err, { phaseOnFailure = 'failed' } = {}) => {
    const message = err?.message || ''
    const friendly = friendlySpiritAnimalError(message)
    setError(friendly)
    if (isSpiritAnimalAuthError(message)) {
      clearSession()
    }
    if (phaseOnFailure) {
      setPhase(phaseOnFailure)
      setProcessingPurpose(null)
    }
    return friendly
  }, [clearSession])

  const clearTransitionTimers = useCallback(() => {
    transitionTimersRef.current.forEach((timer) => window.clearTimeout(timer))
    transitionTimersRef.current = []
  }, [])

  const queueTransitionTimer = useCallback((fn, delayMs) => {
    const timer = window.setTimeout(fn, delayMs)
    transitionTimersRef.current.push(timer)
    return timer
  }, [])

  const refreshReading = useCallback(async () => {
    const latest = await fetchMySpiritAnimalReadingWithRetry()
    setReading(latest)
    return latest
  }, [])

  function applyReadingState(existing, purpose = null) {
    setReading(existing)
    if (!existing) {
      setPhase('intro')
      setProcessingPurpose(null)
      setError('')
      return
    }
    if (existing.status === 'FAILED') {
      setPhase('failed')
      setError(friendlySpiritAnimalError(existing.errorMessage))
      setProcessingPurpose(null)
      return
    }
    setError('')
    if (existing.status === 'READY') {
      setPhase('results')
      setProcessingPurpose(null)
      return
    }
    if (existing.status === 'AWAITING_ANSWERS') {
      setPhase('questions')
      setProcessingPurpose(null)
      if (purpose === 'questions') {
        setQuestionIndex(0)
        setAnswers([])
        setPanelState('open')
      }
      return
    }
    if (isSpiritAnimalProcessing(existing.status)) {
      setProcessingPurpose(
        purpose ?? (existing.status === 'GENERATING_QUESTIONS' ? 'questions' : 'mascots'),
      )
      setPhase('processing')
      return
    }
    if (isSpiritAnimalFailed(existing.status)) {
      setPhase('failed')
      setProcessingPurpose(null)
    }
  }

  const handleRegenerateImages = useCallback(async () => {
    setBusy(true)
    setError('')
    try {
      const started = await regenerateSpiritAnimalImages()
      if (started.status === 'READY') {
        applyReadingState(started)
        return
      }
      phaseTimingRef.current = null
      applyReadingState(started, 'mascots')
    } catch (err) {
      handleFlowError(err)
    } finally {
      setBusy(false)
    }
  }, [handleFlowError])

  useEffect(() => {
    let cancelled = false
    void fetchSpiritAnimalJourneyEligibility()
      .then((eligibility) => {
        if (!cancelled) {
          setJourneyEligibility(eligibility)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setJourneyEligibility(null)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const existing = await fetchMySpiritAnimalReadingWithRetry()
        if (cancelled) {
          return
        }
        if (!existing) {
          return
        }
        applyReadingState(existing)
      } catch (err) {
        if (!cancelled) {
          handleFlowError(err)
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [handleFlowError])

  useEffect(() => {
    if (phase !== 'processing' || !reading?.phaseStartedAt || !reading?.estimatedPhaseSeconds) {
      if (phase !== 'processing') {
        phaseTimingRef.current = null
      }
      return
    }
    if (!phaseTimingRef.current) {
      phaseTimingRef.current = {
        phaseStartedAt: reading.phaseStartedAt,
        estimatedPhaseSeconds: reading.estimatedPhaseSeconds,
      }
    }
  }, [phase, reading?.phaseStartedAt, reading?.estimatedPhaseSeconds])

  useEffect(() => {
    if (phase !== 'processing' || !processingPurpose || !reading?.id) {
      return undefined
    }
    let cancelled = false
    void (async () => {
      try {
        const latest = await pollSpiritAnimalReading(refreshReading)
        if (cancelled) {
          return
        }
        if (!latest) {
          setPhase('failed')
          setError(friendlySpiritAnimalError('Mascot image generation failed. Tap Start over to try again.'))
          setProcessingPurpose(null)
          return
        }
        applyReadingState(latest, processingPurpose)
      } catch (err) {
        if (!cancelled) {
          try {
            const recovered = await fetchMySpiritAnimalReadingWithRetry()
            if (!cancelled && recovered) {
              applyReadingState(recovered, processingPurpose)
              return
            }
          } catch {
            // fall through to failed state below
          }
          setError(friendlySpiritAnimalError(err.message))
          setPhase('failed')
          setProcessingPurpose(null)
          if (isSpiritAnimalAuthError(err.message)) {
            clearSession()
          }
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [phase, processingPurpose, reading?.id, refreshReading, clearSession])

  useEffect(() => {
    if (phase !== 'processing') {
      setCountdownSeconds(null)
      return undefined
    }
    const tick = () => {
      const timing = phaseTimingRef.current
      if (!timing) {
        setCountdownSeconds(null)
        return
      }
      setCountdownSeconds(secondsRemainingFromPhase(timing.phaseStartedAt, timing.estimatedPhaseSeconds))
    }
    tick()
    const timer = window.setInterval(tick, 1000)
    return () => window.clearInterval(timer)
  }, [phase])

  useEffect(() => () => clearTransitionTimers(), [clearTransitionTimers])

  useEffect(() => {
    if (phase !== 'results' || !reading?.imagesMissing || regenRequested || busy) {
      return undefined
    }
    setRegenRequested(true)
    void handleRegenerateImages()
    return undefined
  }, [phase, reading, regenRequested, busy, handleRegenerateImages])

  async function handleBegin({ forceRestart = false } = {}) {
    setBusy(true)
    setError('')
    setAnswers([])
    setQuestionIndex(0)
    setPanelState('open')
    setRegenRequested(false)
    try {
      const started = await beginSpiritAnimalReading({ forceRestart })
      phaseTimingRef.current = null
      applyReadingState(started, 'questions')
    } catch (err) {
      handleFlowError(err)
    } finally {
      setBusy(false)
    }
  }

  async function handleResume() {
    setBusy(true)
    setError('')
    try {
      const existing = await fetchMySpiritAnimalReadingWithRetry()
      if (!existing) {
        setPhase('intro')
        setProcessingPurpose(null)
        return
      }
      applyReadingState(existing)
    } catch (err) {
      handleFlowError(err, { phaseOnFailure: 'intro' })
    } finally {
      setBusy(false)
    }
  }

  function handlePickAnswer(answerID) {
    if (busy || panelState !== 'open') {
      return
    }
    const nextAnswers = [...answers, answerID]
    setAnswers(nextAnswers)
    const questions = reading?.cardQuestions ?? []
    if (nextAnswers.length >= questions.length) {
      void handleSubmitAnswers(nextAnswers)
      return
    }
    clearTransitionTimers()
    setPanelState('closing')
    journeyAnchorRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    queueTransitionTimer(() => {
      setQuestionIndex(nextAnswers.length)
      setPanelState('closed')
      queueTransitionTimer(() => {
        setPanelState('opening')
        queueTransitionTimer(() => setPanelState('open'), QUESTION_PANEL_OPEN_MS)
      }, QUESTION_PANEL_PAUSE_MS)
    }, QUESTION_PANEL_CLOSE_MS)
  }

  async function handleSubmitAnswers(finalAnswers) {
    setBusy(true)
    setError('')
    try {
      const submitted = await submitSpiritAnimalAnswers(finalAnswers)
      if (submitted.status === 'READY') {
        applyReadingState(submitted)
        return
      }
      if (isSpiritAnimalFailed(submitted.status)) {
        applyReadingState(submitted)
        return
      }
      phaseTimingRef.current = null
      applyReadingState(submitted, 'mascots')
    } catch (err) {
      handleFlowError(err, { phaseOnFailure: 'questions' })
      setQuestionIndex(finalAnswers.length - 1)
      setAnswers(finalAnswers.slice(0, -1))
    } finally {
      setBusy(false)
    }
  }

  async function handleSelectTotem(totemName) {
    setBusy(true)
    setError('')
    try {
      const updated = await selectSpiritAnimalTotem(totemName)
      acceptSessionUser(updated)
      onComplete?.(updated)
    } catch (err) {
      handleFlowError(err, { phaseOnFailure: 'results' })
    } finally {
      setBusy(false)
    }
  }

  if (phase === 'intro') {
    const journeyBlocked = journeyEligibility && !journeyEligibility.canBegin
    return (
      <Card aria-labelledby="spirit-animal-heading">
        <CardHeader>
          <CardTitle as="h3">Find my spirit animal</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Draw five tarot cards, answer a few symbolic questions, and meet five mascot companions crafted for you.
          </p>
          {journeyBlocked ? (
            <p className="status-message" role="status">
              {formatSpiritAnimalJourneyCooldown(
                journeyEligibility.daysRemaining,
                journeyEligibility.cooldownEndsAt,
              )}
            </p>
          ) : (
            <>
              <p className="text-sm text-muted-foreground">
                Each card lands in a journey slot — five chapters that shape your reading.
              </p>
              <SpiritAnimalSlotGuide compact />
              <div className="flex flex-wrap gap-3">
                <Button type="button" disabled={busy} onClick={handleBegin}>
                  {busy ? 'Drawing cards…' : 'Begin reading'}
                </Button>
                {onCancel ? (
                  <Button type="button" variant="secondary" disabled={busy} onClick={onCancel}>
                    Back
                  </Button>
                ) : null}
              </div>
            </>
          )}
          {error ? <p className="status-message status-message-error">{error}</p> : null}
          {journeyBlocked && onCancel ? (
            <div className="flex flex-wrap gap-3">
              <Button type="button" variant="secondary" disabled={busy} onClick={onCancel}>
                Back
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    )
  }

  if (phase === 'processing') {
    const waitingForQuestions = processingPurpose === 'questions'
    const countdownLabel = formatPhaseCountdown(countdownSeconds) ?? 'Hang tight…'
    return (
      <Card aria-live="polite">
        <CardHeader>
          <CardTitle as="h3">{waitingForQuestions ? 'Reading the cards' : 'Summoning your companions'}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            {waitingForQuestions
              ? 'Five cards are being drawn — one for each chapter of your journey.'
              : 'Your mascots are taking shape. This can take a minute.'}
          </p>
          <p className="font-mono-display text-lg text-primary" role="status" aria-live="polite">
            {countdownLabel}
          </p>
          {waitingForQuestions ? (
            <>
              <p className="text-sm text-muted-foreground">
                While the cards speak, here is what each slot asks about:
              </p>
              <SpiritAnimalSlotGuide />
            </>
          ) : null}
          {error ? <p className="status-message status-message-error">{error}</p> : null}
          {(countdownSeconds != null && countdownSeconds <= 0) || error ? (
            <div className="flex flex-wrap gap-3">
              <Button type="button" disabled={busy} onClick={handleResume}>
                {busy ? 'Checking…' : 'Check again'}
              </Button>
              <Button type="button" variant="secondary" disabled={busy} onClick={() => handleBegin({ forceRestart: true })}>
                Start over
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    )
  }

  if (phase === 'failed') {
    return (
      <Card>
        <CardHeader>
          <CardTitle as="h3">Reading interrupted</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="status-message status-message-error">{friendlySpiritAnimalError(error || reading?.errorMessage)}</p>
          <div className="flex flex-wrap gap-3">
            <Button type="button" disabled={busy} onClick={handleResume}>
              {busy ? 'Checking…' : 'Check again'}
            </Button>
            <Button type="button" variant="secondary" disabled={busy} onClick={() => handleBegin({ forceRestart: true })}>
              Start over
            </Button>
            {onCancel ? (
              <Button type="button" variant="secondary" disabled={busy} onClick={onCancel}>
                Back
              </Button>
            ) : null}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (phase === 'questions') {
    const questions = reading?.cardQuestions ?? []
    const current = questions[questionIndex]
    if (!current) {
      return (
        <Card>
          <CardContent>
            <p className="status-message" role="status">Loading questions…</p>
          </CardContent>
        </Card>
      )
    }
    const slotPrompt = slotPromptForKey(current.slot)
    const answersDisabled = busy || panelState !== 'open'
    return (
      <Card aria-labelledby="spirit-question-heading">
        <CardContent>
          <div ref={journeyAnchorRef} className="flex flex-col gap-3">
            <p className="text-xs font-medium text-muted-foreground">
              Question {questionIndex + 1} of {questions.length}
            </p>
            <SpiritAnimalSlotGuide compact highlightKey={current.slot} />
          </div>

          <div
            className={cn(
              'flex flex-col gap-4 transition-opacity duration-300',
              (panelState === 'closing' || panelState === 'closed') && 'pointer-events-none opacity-0',
              panelState === 'opening' && 'opacity-0',
              panelState === 'open' && 'opacity-100',
            )}
          >
            <div className="flex flex-col gap-2 rounded-xl border border-border bg-muted/40 p-3">
              <p className="text-sm text-muted-foreground">
                <span className="font-semibold text-foreground">The {current.slotName}</span>
                {' '}asks about {slotPrompt.replace(/\.$/, '')}.
              </p>

              <p className="text-sm text-muted-foreground">
                <span className="font-semibold text-foreground">Your card: {current.card}</span>
                {current.cardMeaningInGeneral ? (
                  <>
                    {' '}
                    {current.cardMeaningInGeneral}
                  </>
                ) : null}
              </p>

              {current.cardMeaningForSlot ? (
                <p className="text-sm text-muted-foreground">
                  <span className="font-semibold text-foreground">In the {current.slotName} position</span>
                  {' '}
                  {current.cardMeaningForSlot}
                </p>
              ) : null}
            </div>

            <h3 id="spirit-question-heading" className="font-heading text-lg font-semibold">
              {current.question}
            </h3>

            <ul className="flex flex-col gap-2" role="list">
              {current.answers.map((answer) => (
                <li key={answer.id}>
                  <button
                    type="button"
                    className="flex w-full items-center gap-3 rounded-full border border-border bg-muted/40 px-4 py-2.5 text-left transition-colors hover:border-primary hover:bg-primary/10"
                    disabled={answersDisabled}
                    onClick={() => handlePickAnswer(answer.id)}
                  >
                    <span className="font-mono-display text-2xs text-muted-foreground">{answer.id}</span>
                    <span className="text-sm text-foreground">{answer.label}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
          {error ? <p className="status-message status-message-error">{error}</p> : null}
        </CardContent>
      </Card>
    )
  }

  const totems = reading?.totems ?? []
  return (
    <Card aria-labelledby="spirit-results-heading">
      <CardHeader>
        <CardTitle as="h3">Your spirit animals</CardTitle>
      </CardHeader>
      <CardContent>
        {reading?.personality?.overview ? (
          <p className="text-sm text-muted-foreground">{reading.personality.overview}</p>
        ) : null}
        {reading?.mascotOverview ? <p className="text-sm text-muted-foreground">{reading.mascotOverview}</p> : null}
        {reading?.imagesMissing && phase === 'results' ? (
          <p className="text-sm text-muted-foreground" role="status">Restoring mascot images…</p>
        ) : null}
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-4" role="list">
          {totems.map((totem) => (
            <li key={totem.name} className="flex flex-col gap-2 rounded-2xl border border-border bg-muted/40 p-4">
              {totem.imageUrl ? (
                <img src={totem.imageUrl} alt="" className="aspect-square w-full rounded-xl object-cover" />
              ) : null}
              <div className="flex flex-col gap-1.5">
                <h4 className="font-heading text-base font-semibold">{totem.name}</h4>
                {totem.affinity ? <p className="text-xs font-medium text-primary">{totem.affinity}</p> : null}
                <p className="text-sm text-muted-foreground">{totem.personalitySummary}</p>
                {totem.whyChooseThisAvatar ? (
                  <p className="text-xs text-muted-foreground">{totem.whyChooseThisAvatar}</p>
                ) : null}
                <Button type="button" variant="secondary" disabled={busy} onClick={() => handleSelectTotem(totem.name)}>
                  Choose {totem.name}
                </Button>
              </div>
            </li>
          ))}
        </ul>
        {error ? <p className="status-message status-message-error">{error}</p> : null}
      </CardContent>
    </Card>
  )
}
