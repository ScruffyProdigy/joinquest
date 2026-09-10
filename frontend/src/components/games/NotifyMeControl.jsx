import { useCallback, useEffect, useRef, useState } from 'react'
import {
  NOTIFY_BLOCKED,
  NOTIFY_FAILED,
  NOTIFY_ME,
  NOTIFY_ME_IOS,
  NOTIFY_ME_VALUE,
  NOTIFY_OFF,
  NOTIFY_ON,
  NOTIFY_ON_VALUE,
} from '../../lib/playerCopy'
import {
  disablePushNotifications,
  enablePushNotifications,
  fetchPushCapability,
  pushBlockedReason,
} from '../../lib/push'
import { isInstallPromptSnoozed, snoozeInstallPrompt } from '../../lib/installPrompt'
import { Button } from '../ui/button'
import InstallSheet from './InstallSheet'

/* How long before the control is promoted from a quiet option to the obvious
   one. Showing it and pressing it are separate: it is present from the moment
   the player joins, because someone who pockets their phone at 8s must have
   been offered something. */
const PROMOTE_AFTER_MS = 20000

export default function NotifyMeControl({ disabled = false }) {
  const [capability, setCapability] = useState(null)
  const [blocked, setBlocked] = useState(null)
  const [promoted, setPromoted] = useState(false)
  const [showingInstall, setShowingInstall] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)
  const mounted = useRef(true)

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    fetchPushCapability()
      .then((next) => {
        if (!cancelled) {
          setCapability(next)
          setBlocked(pushBlockedReason())
        }
      })
      .catch(() => {
        // Treat a failed lookup as "no push here" rather than showing a
        // control that may not work.
        if (!cancelled) {
          setCapability({ publicKey: null, reachable: false, subscriptionCount: 0 })
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    const timer = setTimeout(() => setPromoted(true), PROMOTE_AFTER_MS)
    return () => clearTimeout(timer)
  }, [])

  const enable = useCallback(async () => {
    setBusy(true)
    setFailed(false)
    try {
      const result = await enablePushNotifications()
      if (!mounted.current) {
        return
      }
      if (result?.blocked) {
        setBlocked(result.blocked)
        if (result.blocked === 'needs-install') {
          setShowingInstall(true)
        }
        return
      }
      setCapability(result)
      setBlocked(null)
    } catch {
      if (mounted.current) {
        setFailed(true)
      }
    } finally {
      if (mounted.current) {
        setBusy(false)
      }
    }
  }, [])

  const disable = useCallback(async () => {
    setBusy(true)
    try {
      const result = await disablePushNotifications()
      if (mounted.current && result) {
        setCapability(result)
      }
    } finally {
      if (mounted.current) {
        setBusy(false)
      }
    }
  }, [])

  // Nothing on this deployment can send, so there is no control to offer.
  if (!capability?.publicKey) {
    return null
  }

  if (capability.reachable) {
    return (
      <div className="notify-me notify-me--on">
        <p className="notify-me__state">{NOTIFY_ON}</p>
        <p className="notify-me__value">{NOTIFY_ON_VALUE}</p>
        <Button
          type="button"
          variant="secondary"
          className="notify-me__off"
          onClick={disable}
          disabled={busy}
        >
          {NOTIFY_OFF}
        </Button>
      </div>
    )
  }

  // Sticky, so there is nothing to press. Say where it can be undone instead.
  if (blocked === 'denied') {
    return (
      <p className="notify-me__blocked" role="status">
        {NOTIFY_BLOCKED}
      </p>
    )
  }

  if (blocked === 'unsupported') {
    return null
  }

  const needsInstall = blocked === 'needs-install'

  return (
    <div className={`notify-me${promoted ? ' notify-me--promoted' : ''}`}>
      <Button
        type="button"
        variant={promoted ? 'default' : 'secondary'}
        className="notify-me__action"
        onClick={() => {
          if (needsInstall) {
            setShowingInstall(true)
            return
          }
          void enable()
        }}
        disabled={disabled || busy}
      >
        {needsInstall ? NOTIFY_ME_IOS : NOTIFY_ME}
      </Button>
      {promoted ? <p className="notify-me__value">{NOTIFY_ME_VALUE}</p> : null}
      {failed ? (
        <p className="notify-me__error" role="alert">
          {NOTIFY_FAILED}
        </p>
      ) : null}

      {/* Mounted only while open, so the sheet costs nothing for the players
          who never see it. */}
      {showingInstall && !isInstallPromptSnoozed() ? (
        <InstallSheet
          open
          onOpenChange={setShowingInstall}
          onDismiss={() => {
            snoozeInstallPrompt()
            setShowingInstall(false)
          }}
        />
      ) : null}
    </div>
  )
}
