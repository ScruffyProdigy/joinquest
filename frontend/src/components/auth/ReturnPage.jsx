import { useEffect, useState } from 'react'
import { fetchReturnDestination } from '../../lib/return'
import { APP_NAME } from '../../lib/brand'
import { Button } from '../ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'

function matchIdFromLocation() {
  const params = new URLSearchParams(window.location.search)
  return params.get('match')?.trim() || ''
}

export default function ReturnPage() {
  const [status, setStatus] = useState('loading')
  const [message, setMessage] = useState('Taking you back…')

  useEffect(() => {
    let cancelled = false
    const matchId = matchIdFromLocation()

    fetchReturnDestination(matchId || null)
      .then((dest) => {
        if (cancelled) return
        const path = dest?.path?.trim() || '/'
        const safePath = path.startsWith('/') && !path.startsWith('//') ? path : '/'
        setStatus('success')
        setMessage('Redirecting…')
        window.history.replaceState({}, '', safePath)
        window.location.assign(safePath)
      })
      .catch((error) => {
        if (cancelled) return
        setStatus('error')
        setMessage(error.message || 'Could not resolve return destination')
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
          <CardTitle>Welcome back</CardTitle>
        </CardHeader>
        <CardContent>
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
