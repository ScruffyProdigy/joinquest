import { useEffect, useState } from 'react'
import { oauthErrorMessage } from '../../lib/oauth'
import { Link } from '../ui/link'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'

export default function OAuthCompletePage() {
  const [message, setMessage] = useState('Something went wrong during sign-in.')

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const error = params.get('error')
    if (error) {
      setMessage(oauthErrorMessage(error))
      window.history.replaceState({}, '', '/auth/oauth/complete')
    }
  }, [])

  return (
    <main className="flex min-h-screen flex-col items-center justify-center bg-background p-6 text-foreground">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle as="h1" className="font-heading text-xl font-semibold">
            Sign-in
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="status-message status-message-error" role="alert">
            {message}
          </p>
          <Link className="self-start" href="/">
            Back to home
          </Link>
        </CardContent>
      </Card>
    </main>
  )
}
