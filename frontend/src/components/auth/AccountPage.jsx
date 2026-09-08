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
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { Link } from '../ui/link'
import { Input } from '../ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import { useAuth } from './AuthProvider'
import UserSessionCard from './UserSessionCard'
import CodeInput from './CodeInput'
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
  const { user, acceptSessionUser, refreshSession, getSessionGeneration } = useAuth()
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
    // Logging out mid-flight makes this response worthless: it describes a
    // session that is already gone, and applying it puts the signed-out user
    // back on the page (JQ-205).
    const sessionGeneration = getSessionGeneration()
    setLoading(true)
    setError('')
    try {
      const next = await fetchMyAccount()
      if (sessionGeneration !== getSessionGeneration()) {
        return
      }
      setAccount(next)
      if (next?.user) {
        acceptSessionUser(next.user, { sessionGeneration })
      }
    } catch (err) {
      if (sessionGeneration !== getSessionGeneration()) {
        return
      }
      setError(err.message || 'Could not load account settings')
    } finally {
      setLoading(false)
    }
  }, [acceptSessionUser, getSessionGeneration])

  useEffect(() => {
    if (user) {
      void loadAccount()
    } else {
      // Nothing below the session card should outlive the session it described.
      setAccount(null)
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
      <main className="flex min-h-screen flex-col items-center justify-center bg-background p-6 text-foreground">
        <Card className="w-full max-w-lg">
          <CardHeader>
            <CardTitle as="h1" className="font-heading text-xl font-semibold">
              Account settings
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <p className="text-sm text-muted-foreground">Sign in to manage your account.</p>
            <Link className="self-start" href="/">
              Back to home
            </Link>
          </CardContent>
        </Card>
      </main>
    )
  }

  if (loading && !account) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center bg-background p-6 text-foreground">
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
    <main className="flex min-h-screen flex-col items-center bg-background p-6 text-foreground">
      <div className="flex w-full max-w-lg flex-col gap-6">
        {/* Display name, avatar, and log out — these used to sit on the home page. */}
        <UserSessionCard user={user} showAccountLink={false} />

        <Card className="w-full">
          <CardHeader className="flex items-baseline justify-between">
            <CardTitle as="h1" className="font-heading text-xl font-semibold">
              Account settings
            </CardTitle>
            <Link href="/">Back to home</Link>
          </CardHeader>

          <CardContent className="flex flex-col gap-4">
            {user.isGuest ? <p className="text-sm text-amber-400">{GUEST_ACCOUNT_PROMPT}</p> : null}
            {error ? <p className="status-message status-message-error">{error}</p> : null}

            <section aria-labelledby="linked-emails-heading" className="flex flex-col gap-3">
              <h2 id="linked-emails-heading" className="font-heading text-base font-semibold">
                Email addresses
              </h2>
              {emails.length === 0 ? (
                <p className="text-sm text-muted-foreground">No email linked yet.</p>
              ) : (
                <ul className="flex flex-col gap-2">
                  {emails.map((item) => (
                    <li
                      key={item.id}
                      className="flex items-center justify-between gap-3 rounded-lg border border-border bg-muted/40 px-3 py-2"
                    >
                      <div>
                        <strong className="text-sm text-foreground">{item.email}</strong>
                        {item.isPrimary ? (
                          <span className="ml-2 text-xs font-semibold text-primary">Primary</span>
                        ) : null}
                      </div>
                      <div className="flex flex-wrap justify-end gap-3">
                        {!item.isPrimary ? (
                          <Link
                            className="text-xs"
                            onClick={() => void handleSetPrimary(item.id)}
                            disabled={actionStatus === 'loading'}
                          >
                            Make primary
                          </Link>
                        ) : null}
                        {canRemove ? (
                          <Link
                            className="text-xs"
                            onClick={() => void handleRemoveEmail(item.id)}
                            disabled={actionStatus === 'loading'}
                          >
                            Remove
                          </Link>
                        ) : null}
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section aria-labelledby="add-email-heading" className="flex flex-col gap-3">
              <h2 id="add-email-heading" className="font-heading text-base font-semibold">
                Add email
              </h2>
              {linkMessage ? (
                <p className={linkStatus === 'error' ? 'status-message status-message-error' : 'status-message'} role="status">
                  {linkMessage}
                </p>
              ) : null}

              {linkStep === 'merge-confirm' && mergePreview ? (
                <div className="flex flex-col gap-3 rounded-lg border border-amber-500/25 bg-amber-500/10 p-3" role="alert">
                  <p className="text-sm text-amber-100">
                    {formatMergeWarning(mergePreview.mergeSourceDisplayName, user.displayName)}
                  </p>
                  <div className="flex flex-wrap gap-3">
                    <Button onClick={() => void handleConfirmMerge()} disabled={linkStatus === 'loading'}>
                      {linkStatus === 'loading' ? 'Sending…' : MERGE_CONFIRM}
                    </Button>
                    <Button variant="secondary" onClick={handleCancelMerge} disabled={linkStatus === 'loading'}>
                      {MERGE_CANCEL}
                    </Button>
                  </div>
                </div>
              ) : null}

              {linkStep === 'verify' ? (
                <form className="flex flex-col gap-3" onSubmit={handleVerifyLink}>
                  <label htmlFor="link-code" className="text-sm font-medium text-muted-foreground">
                    Verification code
                  </label>
                  <CodeInput
                    ref={codeInputRef}
                    id="link-code"
                    type="text"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    maxLength={6}
                    value={code}
                    onChange={(event) => setCode(normalizeCode(event.target.value))}
                  />
                  <Button type="submit" disabled={linkStatus === 'loading' || normalizeCode(code).length !== 6}>
                    {linkStatus === 'loading' ? 'Verifying…' : 'Verify email'}
                  </Button>
                </form>
              ) : linkStep === 'email' ? (
                <form className="flex flex-col gap-3" onSubmit={handleRequestLink}>
                  <label htmlFor="link-email" className="text-sm font-medium text-muted-foreground">
                    Email
                  </label>
                  <Input
                    id="link-email"
                    type="email"
                    autoComplete="email"
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                    placeholder="you@example.com"
                    disabled={linkStatus === 'loading'}
                  />
                  <Button type="submit" disabled={linkStatus === 'loading' || email.trim() === ''}>
                    {linkStatus === 'loading' ? 'Checking…' : 'Send verification code'}
                  </Button>
                </form>
              ) : null}
            </section>

            <section aria-labelledby="linked-identities-heading" className="flex flex-col gap-3">
              <h2 id="linked-identities-heading" className="font-heading text-base font-semibold">
                Connected accounts
              </h2>
              {oauthMessage ? (
                <p className="status-message" role="status">
                  {oauthMessage}
                </p>
              ) : null}

              {oauthMergePreview ? (
                <div className="flex flex-col gap-3 rounded-lg border border-amber-500/25 bg-amber-500/10 p-3" role="alert">
                  <p className="text-sm text-amber-100">
                    {formatMergeWarning(oauthMergePreview.mergeSourceDisplayName, user.displayName)}
                  </p>
                  <div className="flex flex-wrap gap-3">
                    <Button onClick={handleConfirmOAuthMerge}>{MERGE_CONFIRM}</Button>
                    <Button variant="secondary" onClick={handleCancelOAuthMerge}>
                      {MERGE_CANCEL}
                    </Button>
                  </div>
                </div>
              ) : null}

              {identities.length === 0 ? (
                <p className="text-sm text-muted-foreground">No social accounts linked yet.</p>
              ) : (
                <ul className="flex flex-col gap-2">
                  {identities.map((item) => (
                    <li
                      key={item.id}
                      className="flex items-center justify-between gap-3 rounded-lg border border-border bg-muted/40 px-3 py-2"
                    >
                      <div>
                        <strong className="text-sm text-foreground">{providerLabel(item.provider)}</strong>
                        {item.email ? (
                          <span className="ml-2 block text-xs text-muted-foreground">{item.email}</span>
                        ) : null}
                      </div>
                      {canRemove ? (
                        <Link
                          className="text-xs"
                          onClick={() => void handleRemoveIdentity(item.id)}
                          disabled={actionStatus === 'loading'}
                        >
                          Remove
                        </Link>
                      ) : null}
                    </li>
                  ))}
                </ul>
              )}

              {connectProviders.length > 0 ? (
                <div className="flex flex-col gap-2">
                  <p className="text-sm text-muted-foreground">Connect another sign-in method:</p>
                  <div className="flex flex-col gap-2">
                    {connectProviders.map((provider) => (
                      <Button
                        key={provider}
                        variant="outline"
                        className={cn(
                          'w-full justify-start gap-3',
                          provider === 'GOOGLE' && '[&_svg]:size-5',
                          provider === 'DISCORD' && '[&_svg]:size-6',
                        )}
                        onClick={() => handleConnectProvider(provider)}
                        aria-label={`Connect ${oauthLabel(provider)}`}
                      >
                        <OAuthProviderIcon provider={provider} />
                        Connect {oauthLabel(provider)}
                      </Button>
                    ))}
                  </div>
                </div>
              ) : null}
            </section>
          </CardContent>
        </Card>
      </div>
    </main>
  )
}
