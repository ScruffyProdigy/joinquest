import { useCallback, useEffect, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import {
  completeLinkEmailWithCode,
  fetchMyAccount,
  previewLinkEmail,
  removeLinkedEmail,
  requestLinkEmail,
  setPrimaryEmail,
} from '../../lib/auth'
import {
  fetchEnabledOAuthProviders,
  oauthErrorMessage,
  removeLinkedIdentity,
  startOAuthLink,
} from '../../lib/oauth'
import {
  formatMergeWarning,
  GUEST_ACCOUNT_PROMPT,
  MERGE_CANCEL,
  MERGE_CONFIRM,
} from '../../lib/playerCopy'
import { useAuth } from './AuthProvider'
import { focusCodeInput } from './focusCodeInput'
import OAuthProviderIcon, { oauthProviderLabel as oauthLabel } from './OAuthProviderIcon'

function normalizeCode(value) {
  return value.replace(/\D/g, '').slice(0, 6)
}

function providerLabel(provider) {
  switch (provider) {
    case 'GOOGLE':
      return 'Google'
    case 'DISCORD':
      return 'Discord'
    case 'APPLE':
      return 'Apple'
    case 'FACEBOOK':
      return 'Facebook'
    default:
      return provider
  }
}

export default function AccountPage() {
  const { user, acceptSessionUser, refreshSession } = useAuth()
  const [account, setAccount] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [linkStep, setLinkStep] = useState('email')
  const [linkStatus, setLinkStatus] = useState('idle')
  const [linkMessage, setLinkMessage] = useState('')
  const [mergePreview, setMergePreview] = useState(null)
  const [actionStatus, setActionStatus] = useState('idle')
  const [enabledProviders, setEnabledProviders] = useState([])
  const [oauthMergePreview, setOauthMergePreview] = useState(null)
  const [oauthMessage, setOauthMessage] = useState('')
  const codeInputRef = useRef(null)

  const loadAccount = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const next = await fetchMyAccount()
      setAccount(next)
      if (next?.user) {
        acceptSessionUser(next.user)
      }
    } catch (err) {
      setError(err.message || 'Could not load account settings')
    } finally {
      setLoading(false)
    }
  }, [acceptSessionUser])

  useEffect(() => {
    if (user) {
      void loadAccount()
    } else {
      setLoading(false)
    }
  }, [user, loadAccount])

  useEffect(() => {
    let cancelled = false
    async function loadProviders() {
      try {
        const providers = await fetchEnabledOAuthProviders()
        if (!cancelled) {
          setEnabledProviders(providers)
        }
      } catch {
        if (!cancelled) {
          setEnabledProviders([])
        }
      }
    }
    void loadProviders()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const error = params.get('error')
    if (error) {
      setOauthMessage(oauthErrorMessage(error))
    } else if (params.get('linked') === '1') {
      setOauthMessage('Connected account linked.')
      void refreshSession({ silent: true })
      void loadAccount()
    }
    if (params.get('oauth_merge') === '1') {
      setOauthMergePreview({
        provider: (params.get('provider') || '').toUpperCase(),
        mergeSourceDisplayName: params.get('source') || 'another account',
      })
    }
    if (params.toString()) {
      window.history.replaceState({}, '', '/account')
    }
  }, [loadAccount, refreshSession])

  async function sendVerificationEmail() {
    setLinkStatus('loading')
    setLinkMessage('Sending verification email…')
    try {
      await requestLinkEmail(email.trim())
      flushSync(() => {
        setLinkStep('verify')
        setCode('')
        setLinkStatus('idle')
        setLinkMessage('Enter the 6-digit code we sent to verify this email.')
      })
      focusCodeInput(codeInputRef.current)
    } catch (err) {
      setLinkStatus('error')
      setLinkMessage(err.message || 'Could not send verification email')
    }
  }

  async function handleRequestLink(event) {
    event.preventDefault()
    const trimmedEmail = email.trim()
    if (!trimmedEmail || linkStatus === 'loading') {
      return
    }

    setLinkStatus('loading')
    setLinkMessage('')
    setMergePreview(null)
    try {
      const preview = await previewLinkEmail(trimmedEmail)
      if (preview.willMergeAccounts) {
        setMergePreview(preview)
        setLinkStep('merge-confirm')
        setLinkStatus('idle')
        return
      }
      await sendVerificationEmail()
    } catch (err) {
      setLinkStatus('error')
      setLinkMessage(err.message || 'Could not check this email')
    }
  }

  async function handleConfirmMerge() {
    await sendVerificationEmail()
  }

  function handleCancelMerge() {
    setMergePreview(null)
    setLinkStep('email')
    setLinkMessage('')
    setLinkStatus('idle')
  }

  async function handleVerifyLink(event) {
    event.preventDefault()
    const normalizedCode = normalizeCode(code)
    if (normalizedCode.length !== 6 || linkStatus === 'loading') {
      return
    }
    setLinkStatus('loading')
    setLinkMessage('')
    try {
      const updatedUser = await completeLinkEmailWithCode(
        email.trim(),
        normalizedCode,
        Boolean(mergePreview?.willMergeAccounts),
      )
      acceptSessionUser(updatedUser)
      void refreshSession({ silent: true })
      setLinkStep('email')
      setEmail('')
      setCode('')
      setMergePreview(null)
      setLinkMessage('Email linked.')
      await loadAccount()
    } catch (err) {
      setLinkStatus('error')
      setLinkMessage(err.message || 'Invalid or expired code')
      focusCodeInput(codeInputRef.current)
    } finally {
      setLinkStatus('idle')
    }
  }

  async function handleRemoveEmail(emailId) {
    if (actionStatus === 'loading') {
      return
    }
    setActionStatus('loading')
    setError('')
    try {
      const updatedUser = await removeLinkedEmail(emailId)
      acceptSessionUser(updatedUser)
      await loadAccount()
    } catch (err) {
      setError(err.message || 'Could not remove email')
    } finally {
      setActionStatus('idle')
    }
  }

  async function handleSetPrimary(emailId) {
    if (actionStatus === 'loading') {
      return
    }
    setActionStatus('loading')
    setError('')
    try {
      const updatedUser = await setPrimaryEmail(emailId)
      acceptSessionUser(updatedUser)
      await loadAccount()
    } catch (err) {
      setError(err.message || 'Could not update primary email')
    } finally {
      setActionStatus('idle')
    }
  }

  async function handleRemoveIdentity(identityId) {
    if (actionStatus === 'loading') {
      return
    }
    setActionStatus('loading')
    setError('')
    try {
      await removeLinkedIdentity(identityId)
      await loadAccount()
      void refreshSession({ silent: true })
    } catch (err) {
      setError(err.message || 'Could not remove connected account')
    } finally {
      setActionStatus('idle')
    }
  }

  function handleConnectProvider(provider) {
    startOAuthLink(provider)
  }

  function handleConfirmOAuthMerge() {
    if (!oauthMergePreview?.provider) {
      return
    }
    startOAuthLink(oauthMergePreview.provider, true)
  }

  function handleCancelOAuthMerge() {
    setOauthMergePreview(null)
    setOauthMessage('')
  }

  if (!user) {
    return (
      <main className="app-shell auth-page">
        <section className="panel-card">
          <h1>Account settings</h1>
          <p className="panel-copy">Sign in to manage your account.</p>
          <a className="auth-link" href="/">
            Back to home
          </a>
        </section>
      </main>
    )
  }

  if (loading && !account) {
    return (
      <main className="app-shell auth-page">
        <p className="status-message" role="status">
          Loading account…
        </p>
      </main>
    )
  }

  const emails = account?.emails ?? []
  const identities = account?.identities ?? []
  const canRemove = (account?.signInMethodCount ?? 0) > 1
  const linkedProviderSet = new Set(identities.map((item) => item.provider))
  const connectProviders = enabledProviders.filter((provider) => !linkedProviderSet.has(provider))

  return (
    <main className="app-shell auth-page account-page">
      <section className="panel-card account-page__card">
        <header className="account-page__header">
          <h1>Account settings</h1>
          <a className="auth-link" href="/">
            Back to home
          </a>
        </header>

        {user.isGuest ? <p className="account-page__guest-note">{GUEST_ACCOUNT_PROMPT}</p> : null}
        {error ? <p className="status-message status-message-error">{error}</p> : null}

        <section aria-labelledby="linked-emails-heading">
          <h2 id="linked-emails-heading">Email addresses</h2>
          {emails.length === 0 ? (
            <p className="panel-copy">No email linked yet.</p>
          ) : (
            <ul className="account-method-list">
              {emails.map((item) => (
                <li key={item.id} className="account-method-list__item">
                  <div>
                    <strong>{item.email}</strong>
                    {item.isPrimary ? <span className="account-method-list__badge">Primary</span> : null}
                  </div>
                  <div className="account-method-list__actions">
                    {!item.isPrimary ? (
                      <button type="button" className="auth-link-button" onClick={() => void handleSetPrimary(item.id)} disabled={actionStatus === 'loading'}>
                        Make primary
                      </button>
                    ) : null}
                    {canRemove ? (
                      <button type="button" className="auth-link-button" onClick={() => void handleRemoveEmail(item.id)} disabled={actionStatus === 'loading'}>
                        Remove
                      </button>
                    ) : null}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section aria-labelledby="add-email-heading">
          <h2 id="add-email-heading">Add email</h2>
          {linkMessage ? (
            <p className={linkStatus === 'error' ? 'status-message status-message-error' : 'status-message'} role="status">
              {linkMessage}
            </p>
          ) : null}

          {linkStep === 'merge-confirm' && mergePreview ? (
            <div className="account-merge-warning" role="alert">
              <p className="panel-copy">
                {formatMergeWarning(mergePreview.mergeSourceDisplayName, user.displayName)}
              </p>
              <div className="account-merge-warning__actions">
                <button type="button" onClick={() => void handleConfirmMerge()} disabled={linkStatus === 'loading'}>
                  {linkStatus === 'loading' ? 'Sending…' : MERGE_CONFIRM}
                </button>
                <button type="button" className="button-secondary" onClick={handleCancelMerge} disabled={linkStatus === 'loading'}>
                  {MERGE_CANCEL}
                </button>
              </div>
            </div>
          ) : null}

          {linkStep === 'verify' ? (
            <form className="auth-form" onSubmit={handleVerifyLink}>
              <label htmlFor="link-code">Verification code</label>
              <input
                ref={codeInputRef}
                id="link-code"
                type="text"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={6}
                value={code}
                onChange={(event) => setCode(normalizeCode(event.target.value))}
                className="auth-code-input"
              />
              <button type="submit" disabled={linkStatus === 'loading' || normalizeCode(code).length !== 6}>
                {linkStatus === 'loading' ? 'Verifying…' : 'Verify email'}
              </button>
            </form>
          ) : linkStep === 'email' ? (
            <form className="auth-form" onSubmit={handleRequestLink}>
              <label htmlFor="link-email">Email</label>
              <input
                id="link-email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
                disabled={linkStatus === 'loading'}
              />
              <button type="submit" disabled={linkStatus === 'loading' || email.trim() === ''}>
                {linkStatus === 'loading' ? 'Checking…' : 'Send verification code'}
              </button>
            </form>
          ) : null}
        </section>

        <section aria-labelledby="linked-identities-heading">
          <h2 id="linked-identities-heading">Connected accounts</h2>
          {oauthMessage ? (
            <p className="status-message" role="status">
              {oauthMessage}
            </p>
          ) : null}

          {oauthMergePreview ? (
            <div className="account-merge-warning" role="alert">
              <p className="panel-copy">
                {formatMergeWarning(oauthMergePreview.mergeSourceDisplayName, user.displayName)}
              </p>
              <div className="account-merge-warning__actions">
                <button type="button" onClick={handleConfirmOAuthMerge}>
                  {MERGE_CONFIRM}
                </button>
                <button type="button" className="button-secondary" onClick={handleCancelOAuthMerge}>
                  {MERGE_CANCEL}
                </button>
              </div>
            </div>
          ) : null}

          {identities.length === 0 ? (
            <p className="panel-copy">No social accounts linked yet.</p>
          ) : (
            <ul className="account-method-list">
              {identities.map((item) => (
                <li key={item.id} className="account-method-list__item">
                  <div>
                    <strong>{providerLabel(item.provider)}</strong>
                    {item.email ? <span className="account-method-list__meta">{item.email}</span> : null}
                  </div>
                  {canRemove ? (
                    <div className="account-method-list__actions">
                      <button
                        type="button"
                        className="auth-link-button"
                        onClick={() => void handleRemoveIdentity(item.id)}
                        disabled={actionStatus === 'loading'}
                      >
                        Remove
                      </button>
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          )}

          {connectProviders.length > 0 ? (
            <div className="account-oauth-connect">
              <p className="panel-copy">Connect another sign-in method:</p>
              <div className="auth-social__icons">
                {connectProviders.map((provider) => (
                  <button
                    key={provider}
                    type="button"
                    className="auth-social__icon"
                    onClick={() => handleConnectProvider(provider)}
                    aria-label={`Connect ${oauthLabel(provider)}`}
                  >
                    <OAuthProviderIcon provider={provider} />
                  </button>
                ))}
              </div>
            </div>
          ) : null}
        </section>
      </section>
    </main>
  )
}
