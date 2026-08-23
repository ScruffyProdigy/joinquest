// frontend/src/components/auth/UserSessionCard.jsx
import { useEffect, useState } from 'react'
import { logout } from '../../lib/auth'
import { needsProfileSetup } from '../../lib/avatars'
import { fetchSpiritAnimalJourneyEligibility, formatSpiritAnimalJourneyCooldown } from '../../lib/spiritAnimal'
import { ACCOUNT_LINK_LABEL, GUEST_BADGE, GUEST_SPIRIT_ANIMAL_HINT } from '../../lib/playerCopy'
import { cn } from '../../lib/utils'
import { Button } from '../ui/button'
import { Card, CardContent, CardHeader } from '../ui/card'
import { useAuth } from './AuthProvider'
import PlayerProfileEditor from '../avatars/PlayerProfileEditor'
import SpiritAnimalFlow from '../avatars/SpiritAnimalFlow'
import PlayerAvatar from '../avatars/PlayerAvatar'

export default function UserSessionCard({ user, compact = false, showProfileActions = true }) {
  const { clearSession } = useAuth()
  const [status, setStatus] = useState('idle')
  const [error, setError] = useState('')
  const setupRequired = needsProfileSetup(user)
  const [editorOpen, setEditorOpen] = useState(setupRequired)
  const [spiritFlowOpen, setSpiritFlowOpen] = useState(false)
  const [journeyEligibility, setJourneyEligibility] = useState(null)
  const [guestSpiritHint, setGuestSpiritHint] = useState(false)

  useEffect(() => {
    if (!showProfileActions) {
      return undefined
    }
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
  }, [user?.id, showProfileActions])

  const canBeginSpiritAnimal = journeyEligibility?.canBegin === true
  const eligibilityPending = journeyEligibility === null

  useEffect(() => {
    if (needsProfileSetup(user)) {
      setEditorOpen(true)
    }
  }, [user])

  async function handleLogout() {
    setStatus('loading')
    setError('')

    try {
      await logout()
      clearSession()
    } catch (err) {
      setStatus('error')
      setError(err.message || 'Could not log out')
    } finally {
      setStatus((current) => (current === 'loading' ? 'idle' : current))
    }
  }

  function handleSaved(updated) {
    if (!needsProfileSetup(updated)) {
      setEditorOpen(false)
      setSpiritFlowOpen(false)
    }
  }

  function handleSpiritComplete(updated) {
    handleSaved(updated)
    if (updated?.isGuest) {
      setGuestSpiritHint(true)
    }
    void fetchSpiritAnimalJourneyEligibility().then(setJourneyEligibility).catch(() => {})
  }

  return (
    <Card className={cn(compact && 'gap-4 p-4')} aria-labelledby="welcome-heading">
      <CardHeader className="flex-row items-center gap-3 space-y-0">
        <PlayerAvatar user={user} size="md" />
        <div className="flex flex-col gap-0.5">
          <h2 id="welcome-heading" className={cn('font-heading font-semibold', compact ? 'text-base' : 'text-lg')}>
            {setupRequired ? 'Set up your display' : 'Welcome back'}
          </h2>
          {user.isGuest ? <p className="text-xs font-semibold text-amber-400">{GUEST_BADGE}</p> : null}
          {user.email ? <p className="text-sm text-foreground">{user.email}</p> : null}
          {!setupRequired && user.displayName ? (
            <p className="text-sm text-muted-foreground">{user.displayName}</p>
          ) : null}
        </div>
      </CardHeader>

      <CardContent className={cn(compact && 'gap-3')}>
        {spiritFlowOpen ? (
          <SpiritAnimalFlow onComplete={handleSpiritComplete} onCancel={() => setSpiritFlowOpen(false)} />
        ) : editorOpen ? (
          <PlayerProfileEditor
            user={user}
            required={setupRequired}
            onSaved={handleSaved}
            onCancel={setupRequired ? undefined : () => setEditorOpen(false)}
            onBeginSpiritAnimal={
              canBeginSpiritAnimal
                ? () => {
                    setEditorOpen(false)
                    setSpiritFlowOpen(true)
                  }
                : undefined
            }
          />
        ) : showProfileActions ? (
          <div className="flex flex-col gap-2">
            <Button variant="secondary" asChild>
              <a href="/account">{ACCOUNT_LINK_LABEL}</a>
            </Button>
            <Button variant="secondary" onClick={() => setEditorOpen(true)}>
              Change display
            </Button>
            {eligibilityPending ? null : canBeginSpiritAnimal ? (
              <Button
                variant="secondary"
                className="justify-start gap-2 bg-gradient-to-br from-primary/20 to-accent/30"
                onClick={() => setSpiritFlowOpen(true)}
              >
                <svg viewBox="0 0 24 24" className="size-4 fill-primary" aria-hidden="true" focusable="false">
                  <path d="M12 2l1.8 5.6L19.4 9l-5.6 1.4L12 16l-1.8-5.6L4.6 9l5.6-1.4L12 2z" />
                </svg>
                Find my spirit animal
              </Button>
            ) : (
              <p className="text-sm text-muted-foreground">
                {formatSpiritAnimalJourneyCooldown(
                  journeyEligibility?.daysRemaining,
                  journeyEligibility?.cooldownEndsAt,
                )}
              </p>
            )}
          </div>
        ) : null}

        {guestSpiritHint ? (
          <p className="text-sm text-muted-foreground">
            {GUEST_SPIRIT_ANIMAL_HINT}{' '}
            <Button variant="link" className="h-auto p-0" asChild>
              <a href="/account">{ACCOUNT_LINK_LABEL}</a>
            </Button>
          </p>
        ) : null}

        <Button variant="ghost" onClick={() => void handleLogout()} disabled={status === 'loading'}>
          {status === 'loading' ? 'Logging out…' : 'Log out'}
        </Button>
        {error ? (
          <p className="status-message status-message-error" role="status">
            {error}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
