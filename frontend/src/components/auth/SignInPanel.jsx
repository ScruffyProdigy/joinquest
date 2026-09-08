import { useState } from 'react'
import { createGuestSession } from '../../lib/auth'
import { notifyAuthComplete } from '../../lib/authBroadcast'
import {
  JUMP_IN,
  JUMP_IN_HINT,
  SIGN_IN_DIVIDER,
  SIGN_IN_DIVIDER_LABEL,
  SIGN_IN_HEADING,
} from '../../lib/playerCopy'
import { Link } from '../ui/link'
import { Card, CardContent, CardHeader } from '../ui/card'
import { useAuth } from './AuthProvider'
import EmailSignInForm from './EmailSignInForm'
import SocialSignInRow from './SocialSignInRow'

function SparkleBadge() {
  return (
    <span className="flex size-12 items-center justify-center rounded-full bg-gradient-to-br from-primary/30 to-accent/30">
      <svg viewBox="0 0 24 24" className="size-6 fill-primary" aria-hidden="true" focusable="false">
        <path d="M12 2l1.8 5.6L19.4 9l-5.6 1.4L12 16l-1.8-5.6L4.6 9l5.6-1.4L12 2z" />
      </svg>
    </span>
  )
}

export default function SignInPanel({ heading = SIGN_IN_HEADING, showGuestOption = true, next = null }) {
  const { acceptSessionUser, refreshSession } = useAuth()
  const [status, setStatus] = useState('idle')
  const [message, setMessage] = useState('')

  async function handleJumpIn() {
    if (status === 'loading') {
      return
    }
    setStatus('loading')
    setMessage('')
    try {
      const guestUser = await createGuestSession()
      acceptSessionUser(guestUser)
      notifyAuthComplete()
      void refreshSession({ silent: true })
    } catch (error) {
      setStatus('error')
      setMessage(error.message || 'Could not start a guest session')
    } finally {
      setStatus('idle')
    }
  }

  return (
    <Card {...(heading === null ? {} : { 'aria-labelledby': 'sign-in-heading' })}>
      {heading === null ? null : (
        <CardHeader className="flex flex-col items-center gap-3 text-center">
          <SparkleBadge />
          <h2 id="sign-in-heading" className="font-heading text-xl font-semibold">
            {heading}
          </h2>
        </CardHeader>
      )}

      <CardContent className="flex flex-col gap-4">
        <SocialSignInRow next={next} />

        <div className="flex items-center gap-3 text-2xs uppercase tracking-wide text-muted-foreground" role="separator" aria-label={SIGN_IN_DIVIDER_LABEL}>
          <span className="h-px flex-1 bg-border" />
          {SIGN_IN_DIVIDER}
          <span className="h-px flex-1 bg-border" />
        </div>

        <EmailSignInForm />

        {message ? (
          <p className={status === 'error' ? 'status-message status-message-error' : 'status-message'} role="status">
            {message}
          </p>
        ) : null}

        {showGuestOption ? (
          <div className="flex flex-col items-center gap-1 border-t border-border pt-4">
            <Link variant="quiet" onClick={() => void handleJumpIn()} disabled={status === 'loading'}>
              {status === 'loading' ? 'Starting…' : JUMP_IN}
            </Link>
            <p className="text-2xs text-muted-foreground">{JUMP_IN_HINT}</p>
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
