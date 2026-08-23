import { useCallback, useLayoutEffect, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import { completeSignInWithCode, requestSignIn } from '../../lib/auth'
import { notifyAuthComplete } from '../../lib/authBroadcast'
import { useAuth } from './AuthProvider'
import useWaitForSignIn from './useWaitForSignIn'
import { focusCodeInput } from './focusCodeInput'

function normalizeCode(value) {
  return value.replace(/\D/g, '').slice(0, 6)
}

export default function EmailSignInForm() {
  const { refreshSession, acceptSessionUser, user } = useAuth()
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [step, setStep] = useState('email')
  const [status, setStatus] = useState('idle')
  const [message, setMessage] = useState('')
  const codeInputRef = useRef(null)
  const emailRequestStarted = useRef(false)
  const codeSubmitStarted = useRef(false)
  const onVerifyStep = step === 'verify'
  const isSigningIn = status === 'loading' && normalizeCode(code).length === 6

  const handleSignedInElsewhere = useCallback(() => {
    void refreshSession({ silent: true })
  }, [refreshSession])

  useWaitForSignIn({
    enabled: onVerifyStep && !user,
    onSignedIn: handleSignedInElsewhere,
  })

  function focusCodeField() {
    focusCodeInput(codeInputRef.current)
  }

  const emailSentMessage =
    'We sent a 6-digit code and sign-in link. Delivery can take a few minutes—check spam if nothing arrives.'

  function showVerifyStepForEmailRequest() {
    flushSync(() => {
      setStep('verify')
      setCode('')
      setStatus('loading')
      setMessage('Sending sign-in email…')
    })
    focusCodeField()
  }

  function completeEmailRequestSuccess() {
    flushSync(() => {
      setStatus('idle')
      setMessage(emailSentMessage)
    })
    focusCodeField()
  }

  useLayoutEffect(() => {
    if (onVerifyStep && status === 'idle') {
      focusCodeField()
    }
  }, [onVerifyStep, status])

  async function handleEmailContinue(event) {
    event.preventDefault()
    const trimmedEmail = email.trim()
    if (trimmedEmail === '' || emailRequestStarted.current) return

    showVerifyStepForEmailRequest()

    emailRequestStarted.current = true
    try {
      await requestSignIn(trimmedEmail)
      completeEmailRequestSuccess()
    } catch (error) {
      emailRequestStarted.current = false
      flushSync(() => {
        setStep('email')
        setStatus('error')
        setMessage(error.message || 'Could not send sign-in email')
      })
    } finally {
      emailRequestStarted.current = false
    }
  }

  async function submitCodeSignIn(normalizedCode) {
    if (codeSubmitStarted.current || status !== 'idle') {
      return
    }

    codeSubmitStarted.current = true
    setStatus('loading')
    setMessage('')

    try {
      const signedInUser = await completeSignInWithCode(email.trim(), normalizedCode)
      acceptSessionUser(signedInUser)
      notifyAuthComplete()
      void refreshSession({ silent: true })
    } catch (error) {
      setStatus('error')
      setMessage(error.message || 'Invalid or expired code')
      focusCodeField()
    } finally {
      setStatus('idle')
      codeSubmitStarted.current = false
    }
  }

  function handleCodeChange(event) {
    const next = normalizeCode(event.target.value)
    setCode(next)
    if (next.length < 6) {
      codeSubmitStarted.current = false
      return
    }
    if (status === 'idle') {
      void submitCodeSignIn(next)
    }
  }

  async function handleFormSubmit(event) {
    event.preventDefault()

    if (!onVerifyStep) {
      await handleEmailContinue(event)
      return
    }

    const normalizedCode = normalizeCode(code)
    if (normalizedCode.length !== 6) {
      setStatus('error')
      setMessage('Enter the 6-digit code from your email.')
      return
    }

    await submitCodeSignIn(normalizedCode)
  }

  function handleUseDifferentEmail() {
    emailRequestStarted.current = false
    codeSubmitStarted.current = false
    setStep('email')
    setCode('')
    setStatus('idle')
    setMessage('')
  }

  const emailContinueDisabled = status === 'loading' || email.trim() === ''

  if (onVerifyStep) {
    return (
      <div className="auth-email-verify" aria-labelledby="verify-heading">
        <h3 id="verify-heading">Enter your code</h3>
        <p className="panel-copy">
          Check <strong>{email}</strong> for a 6-digit code. Tap the field below to use autofill from Mail or Messages,
          or paste your code.
        </p>

        {message ? (
          <p className={status === 'error' ? 'status-message status-message-error' : 'status-message'} role="status">
            {message}
          </p>
        ) : null}

        <form className="auth-form" onSubmit={handleFormSubmit}>
          <label htmlFor="login-code">Sign-in code</label>
          <input
            ref={codeInputRef}
            id="login-code"
            name="code"
            type="text"
            inputMode="numeric"
            autoFocus
            autoComplete="one-time-code"
            autoCapitalize="off"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="done"
            pattern="\d{6}"
            maxLength={6}
            required
            value={code}
            onChange={handleCodeChange}
            placeholder="123456"
            disabled={isSigningIn}
            className="auth-code-input"
          />
          <button
            type="submit"
            disabled={isSigningIn || status === 'loading' || normalizeCode(code).length !== 6}
          >
            {isSigningIn ? 'Signing in…' : 'Continue'}
          </button>
        </form>

        <button
          type="button"
          className="auth-link-button"
          onClick={handleUseDifferentEmail}
          disabled={isSigningIn}
        >
          Use a different email
        </button>
      </div>
    )
  }

  return (
    <div className="auth-email-form">
      {message ? (
        <p className={status === 'error' ? 'status-message status-message-error' : 'status-message'} role="status">
          {message}
        </p>
      ) : null}

      <form className="auth-form" onSubmit={handleFormSubmit}>
        <label htmlFor="email">Email</label>
        <input
          id="email"
          name="email"
          type="email"
          autoComplete="email"
          required
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          placeholder="you@example.com"
          disabled={status === 'loading'}
          className="auth-email-input"
        />
        <button type="submit" disabled={emailContinueDisabled}>
          {status === 'loading' ? 'Sending…' : 'Continue with email'}
        </button>
      </form>
    </div>
  )
}
