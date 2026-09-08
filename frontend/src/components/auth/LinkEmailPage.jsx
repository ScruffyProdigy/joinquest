import { useEffect, useState } from 'react'
import { completeLinkEmailWithLinkOnce, isMergeConfirmationRequired } from '../../lib/auth'
import { notifyAuthComplete } from '../../lib/authBroadcast'
import { APP_NAME } from '../../lib/brand'
import { formatMergeWarning, MERGE_CANCEL, MERGE_CONFIRM } from '../../lib/playerCopy'
import { Button } from '../ui/button'
import { Link } from '../ui/link'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import { useAuth } from './AuthProvider'

function getTokenFromLocation() {
  const params = new URLSearchParams(window.location.search)
  return params.get('token')?.trim() || ''
}

export default function LinkEmailPage() {
  const { user, acceptSessionUser, refreshSession } = useAuth()
  const [status, setStatus] = useState('loading')
  const [message, setMessage] = useState('Verifying your email…')
  const [token, setToken] = useState('')

  useEffect(() => {
    const linkToken = getTokenFromLocation()
    if (!linkToken) {
      setStatus('error')
      setMessage('Missing verification token. Request a new code from Account settings.')
      return undefined
    }

    setToken(linkToken)
    let cancelled = false

    async function attemptLink(confirmMerge) {
      setStatus('loading')
      setMessage(confirmMerge ? 'Merging accounts…' : 'Verifying your email…')
      try {
        const updatedUser = await completeLinkEmailWithLinkOnce(linkToken, confirmMerge)
        if (cancelled) {
          return
        }
        acceptSessionUser(updatedUser)
        notifyAuthComplete()
        void refreshSession({ silent: true })
        setStatus('success')
        setMessage('Email linked. Redirecting…')
        window.history.replaceState({}, '', '/account')
        window.location.assign('/account')
      } catch (error) {
        if (cancelled) {
          return
        }
        if (isMergeConfirmationRequired(error.message)) {
          setStatus('merge')
          setMessage(error.message)
          return
        }
        setStatus('error')
        setMessage(error.message || 'Could not verify this email')
      }
    }

    void attemptLink(false)
    return () => {
      cancelled = true
    }
  }, [acceptSessionUser, refreshSession])

  async function handleConfirmMerge() {
    if (!token) {
      return
    }
    setStatus('loading')
    setMessage('Merging accounts…')
    try {
      const updatedUser = await completeLinkEmailWithLinkOnce(token, true)
      acceptSessionUser(updatedUser)
      notifyAuthComplete()
      void refreshSession({ silent: true })
      setStatus('success')
      setMessage('Email linked. Redirecting…')
      window.history.replaceState({}, '', '/account')
      window.location.assign('/account')
    } catch (error) {
      setStatus('error')
      setMessage(error.message || 'Could not verify this email')
    }
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-background p-6 text-foreground">
      <h1 className="font-heading text-2xl font-bold">{APP_NAME}</h1>
      <Card className="w-full max-w-sm" aria-live="polite">
        <CardHeader>
          <CardTitle as="h2" className="font-heading text-xl font-semibold">
            Link your email
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {status === 'merge' ? (
            <div className="flex flex-col gap-3 rounded-lg border border-amber-500/25 bg-amber-500/10 p-3" role="alert">
              <p className="text-sm text-amber-100">{formatMergeWarning(null, user?.displayName)}</p>
              <div className="flex flex-wrap gap-3">
                <Button onClick={() => void handleConfirmMerge()}>{MERGE_CONFIRM}</Button>
                <Button variant="secondary" asChild>
                  <a href="/account">{MERGE_CANCEL}</a>
                </Button>
              </div>
            </div>
          ) : (
            <p className={status === 'error' ? 'status-message status-message-error' : 'status-message'}>{message}</p>
          )}
          {status === 'error' ? (
            <Link className="self-start" href="/account">
              Back to account settings
            </Link>
          ) : null}
        </CardContent>
      </Card>
    </main>
  )
}
