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
  }, [])

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
  }, [phase, processingPurpose, reading?.id, refreshReading])

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
  }, [phase, reading, regenRequested, busy])

  async function handleRegenerateImages() {
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
  }

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
      <section className="spirit-animal" aria-labelledby="spirit-animal-heading">
        <h3 id="spirit-animal-heading">Find my spirit animal</h3>
        <p className="spirit-animal__lead">
          Draw five tarot cards, answer a few symbolic questions, and meet five mascot companions crafted for you.
        </p>
        {journeyBlocked ? (
          <p className="status-message spirit-animal__hint" role="status">
            {formatSpiritAnimalJourneyCooldown(
              journeyEligibility.daysRemaining,
              journeyEligibility.cooldownEndsAt,
            )}
          </p>
        ) : (
          <>
            <p className="spirit-animal__hint">
              Each card lands in a journey slot — five chapters that shape your reading.
            </p>
            <SpiritAnimalSlotGuide compact />
            <div className="profile-editor__actions">
              <Button type="button" variant="default" disabled={busy} onClick={handleBegin}>
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
          <div className="profile-editor__actions">
            <Button type="button" variant="secondary" disabled={busy} onClick={onCancel}>
              Back
            </Button>
          </div>
        ) : null}
      </section>
    )
  }

  if (phase === 'processing') {
    const waitingForQuestions = processingPurpose === 'questions'
    const countdownLabel = formatPhaseCountdown(countdownSeconds) ?? 'Hang tight…'
    return (
      <section className="spirit-animal" aria-live="polite">
        <h3>{waitingForQuestions ? 'Reading the cards' : 'Summoning your companions'}</h3>
        <p className="spirit-animal__lead">
          {waitingForQuestions
            ? 'Five cards are being drawn — one for each chapter of your journey.'
            : 'Your mascots are taking shape. This can take a minute.'}
        </p>
        <p className="spirit-animal__countdown" role="status" aria-live="polite">
          {countdownLabel}
        </p>
        {waitingForQuestions ? (
          <>
            <p className="spirit-animal__hint">
              While the cards speak, here is what each slot asks about:
            </p>
            <SpiritAnimalSlotGuide />
          </>
        ) : null}
        {error ? <p className="status-message status-message-error">{error}</p> : null}
        {(countdownSeconds != null && countdownSeconds <= 0) || error ? (
          <div className="profile-editor__actions">
            <Button type="button" variant="default" disabled={busy} onClick={handleResume}>
              {busy ? 'Checking…' : 'Check again'}
            </Button>
            <Button type="button" variant="secondary" disabled={busy} onClick={() => handleBegin({ forceRestart: true })}>
              Start over
            </Button>
          </div>
        ) : null}
      </section>
    )
  }

  if (phase === 'failed') {
    return (
      <section className="spirit-animal">
        <h3>Reading interrupted</h3>
        <p className="status-message status-message-error">{friendlySpiritAnimalError(error || reading?.errorMessage)}</p>
        <div className="profile-editor__actions">
          <Button type="button" variant="default" disabled={busy} onClick={handleResume}>
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
      </section>
    )
  }

  if (phase === 'questions') {
    const questions = reading?.cardQuestions ?? []
    const current = questions[questionIndex]
    if (!current) {
      return (
        <section className="spirit-animal">
          <p className="status-message" role="status">Loading questions…</p>
        </section>
      )
    }
    const slotPrompt = slotPromptForKey(current.slot)
    const panelClassName = `spirit-animal__question-panel spirit-animal__question-panel--${panelState}`
    const answersDisabled = busy || panelState !== 'open'
    return (
      <section className="spirit-animal" aria-labelledby="spirit-question-heading">
        <div ref={journeyAnchorRef} className="spirit-animal__journey-anchor">
          <p className="spirit-animal__step">
            Question {questionIndex + 1} of {questions.length}
          </p>
          <SpiritAnimalSlotGuide compact highlightKey={current.slot} />
        </div>

        <div className={panelClassName}>
          <div className="spirit-animal__question-panel-inner">
            <div className="spirit-animal__reading">
              <p className="spirit-animal__reading-block">
                <span className="spirit-animal__reading-label">The {current.slotName}</span>
                {' '}asks about {slotPrompt.replace(/\.$/, '')}.
              </p>

              <p className="spirit-animal__reading-block">
                <span className="spirit-animal__reading-label">Your card: {current.card}</span>
                {current.cardMeaningInGeneral ? (
                  <>
                    {' '}
                    {current.cardMeaningInGeneral}
                  </>
                ) : null}
              </p>

              {current.cardMeaningForSlot ? (
                <p className="spirit-animal__reading-block">
                  <span className="spirit-animal__reading-label">In the {current.slotName} position</span>
                  {' '}
                  {current.cardMeaningForSlot}
                </p>
              ) : null}
            </div>

            <h3 id="spirit-question-heading" className="spirit-animal__question">
              {current.question}
            </h3>

            <ul className="spirit-animal__answers" role="list">
              {current.answers.map((answer) => (
                <li key={answer.id}>
                  <button
                    type="button"
                    className="spirit-animal__answer"
                    disabled={answersDisabled}
                    onClick={() => handlePickAnswer(answer.id)}
                  >
                    <span className="spirit-animal__answer-id">{answer.id}</span>
                    <span>{answer.label}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </div>
        {error ? <p className="status-message status-message-error">{error}</p> : null}
      </section>
    )
  }

  const totems = reading?.totems ?? []
  return (
    <section className="spirit-animal" aria-labelledby="spirit-results-heading">
      <h3 id="spirit-results-heading">Your spirit animals</h3>
      {reading?.personality?.overview ? (
        <p className="spirit-animal__lead">{reading.personality.overview}</p>
      ) : null}
      {reading?.mascotOverview ? <p className="spirit-animal__hint">{reading.mascotOverview}</p> : null}
      {reading?.imagesMissing && phase === 'results' ? (
        <p className="spirit-animal__hint" role="status">Restoring mascot images…</p>
      ) : null}
      <ul className="spirit-animal__totems" role="list">
        {totems.map((totem) => (
          <li key={totem.name} className="spirit-animal__totem">
            {totem.imageUrl ? (
              <img src={totem.imageUrl} alt="" className="spirit-animal__totem-image" />
            ) : null}
            <div>
              <h4>{totem.name}</h4>
              {totem.affinity ? <p className="spirit-animal__affinity">{totem.affinity}</p> : null}
              <p>{totem.personalitySummary}</p>
              {totem.whyChooseThisAvatar ? (
                <p className="spirit-animal__hint">{totem.whyChooseThisAvatar}</p>
              ) : null}
              <button
                type="button"
                className="game-list-button game-list-button-secondary"
                disabled={busy}
                onClick={() => handleSelectTotem(totem.name)}
              >
                Choose {totem.name}
              </button>
            </div>
          </li>
        ))}
      </ul>
      {error ? <p className="status-message status-message-error">{error}</p> : null}
    </section>
  )
}
