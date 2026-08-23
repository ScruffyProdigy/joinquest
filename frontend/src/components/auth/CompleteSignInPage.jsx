import { useEffect, useState } from 'react'
import { completeAuthLinkOnce } from '../../lib/auth'
import { notifyAuthComplete } from '../../lib/authBroadcast'
import { APP_NAME } from '../../lib/brand'
import { Button } from '../ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'

function getTokenFromLocation() {
  const params = new URLSearchParams(window.location.search)
  return params.get('token')?.trim() || ''
}

export default function CompleteSignInPage() {
  const [status, setStatus] = useState('loading')
  const [message, setMessage] = useState('Completing sign-in…')

  useEffect(() => {
    const token = getTokenFromLocation()
    if (!token) {
      setStatus('error')
      setMessage('Missing sign-in token. Request a new sign-in email.')
      return
    }

    let cancelled = false

    completeAuthLinkOnce(token)
      .then(() => {
        if (cancelled) {
          return
        }
        notifyAuthComplete()
        setStatus('success')
        setMessage('Signed in. Redirecting…')
        window.history.replaceState({}, '', '/')
        window.location.assign('/')
      })
      .catch((error) => {
        if (cancelled) {
          return
        }
        setStatus('error')
        setMessage(error.message || 'Could not complete sign-in')
      })

    return () => {
      cancelled = true
    }
  }, [])

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-background p-6 text-foreground">
      <h1 className="font-heading text-2xl font-bold">{APP_NAME}</h1>
      <Card className="w-full max-w-sm" aria-live="polite">
        <CardHeader>
          <CardTitle as="h2" className="font-heading text-xl font-semibold">
            Signing you in
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className={status === 'error' ? 'status-message status-message-error' : 'status-message'}>{message}</p>
          {status === 'error' ? (
            <Button variant="link" className="self-start px-0" asChild>
              <a href="/">Back to home</a>
            </Button>
          ) : null}
        </CardContent>
      </Card>
    </main>
  )
}
