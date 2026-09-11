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
  NOTIFY_RETRY,
  NOTIFY_UNVERIFIED,
  NOTIFY_UNVERIFIED_VALUE,
  NOTIFY_VERIFYING,
} from '../../lib/playerCopy'
import {
  disablePushNotifications,
  enablePushNotifications,
  fetchPushCapability,
  pushBlockedReason,
} from '../../lib/push'
import { isInstallPromptSnoozed, snoozeInstallPrompt } from '../../lib/installPrompt'
import { isLongQueue } from '../../lib/queueLength'
import { Button } from '../ui/button'
import { Link } from '../ui/link'
import InstallSheet from './InstallSheet'

/* How long before the control is promoted from a quiet option to the obvious
   one. Showing it and pressing it are separate: it is present from the moment
   the player joins, because someone who pockets their phone at 8s must have
   been offered something. */
const PROMOTE_AFTER_MS = 20000

export default function NotifyMeControl({
  disabled = false,
  estimatedWaitSeconds = null,
  onReachableChange,
}) {
  const [capability, setCapability] = useState(null)
  const [blocked, setBlocked] = useState(null)
  const [promoted, setPromoted] = useState(false)
  const [showingInstall, setShowingInstall] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failed, setFailed] = useState(false)
  const [unverified, setUnverified] = useState(false)
  const mounted = useRef(true)

  // Short queues never ask. Every hook below still runs, so the estimate
  // arriving mid-wait flips the control on without remounting anything.
  const longQueue = isLongQueue(estimatedWaitSeconds)

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

  // Reported upward rather than kept here: opting in changes the page around
  // this control, not just the control.
  const reachable = capability?.reachable === true
  useEffect(() => {
    onReachableChange?.(reachable)
  }, [reachable, onReachableChange])

  const enable = useCallback(async () => {
    setBusy(true)
    setFailed(false)
    setUnverified(false)
    try {
      const result = await enablePushNotifications()
      if (!mounted.current) {
        return
      }
      if (result?.blocked) {
        // 'unverified' is its own outcome, not an error: permission was
        // granted and a subscription exists. What failed is the only part that
        // mattered -- a push getting through.
        if (result.blocked === 'unverified') {
          setUnverified(true)
          setCapability(result.capability ?? capability)
          return
        }
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
  }, [capability])

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

  // A wait this short is over before leaving the page is worth doing, and the
  // permission prompt only comes round once. See lib/queueLength.js.
  if (!longQueue) {
    return null
  }

  if (capability.reachable) {
    return (
      <div className="notify-me notify-me--on">
        <p className="notify-me__state">{NOTIFY_ON}</p>
        <p className="notify-me__value">{NOTIFY_ON_VALUE}</p>
        <Link variant="quiet" className="notify-me__off" onClick={disable} disabled={busy}>
          {NOTIFY_OFF}
        </Link>
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
  const press = () => {
    if (needsInstall) {
      setShowingInstall(true)
      return
    }
    void enable()
  }

  // The round trip failed, so say so where the offer used to be. Quietly
  // leaving the original control in place would read as an offer still
  // standing.
  if (unverified && !busy) {
    return (
      <div className="notify-me notify-me--unverified">
        <p className="notify-me__state" role="alert">
          {NOTIFY_UNVERIFIED}
        </p>
        <p className="notify-me__value">{NOTIFY_UNVERIFIED_VALUE}</p>
        <Button
          type="button"
          variant="secondary"
          className="notify-me__action"
          onClick={press}
          disabled={disabled}
        >
          {NOTIFY_RETRY}
        </Button>
      </div>
    )
  }

  const label = needsInstall ? NOTIFY_ME_IOS : NOTIFY_ME

  return (
    <div className={`notify-me${promoted ? ' notify-me--promoted' : ''}`}>
      {/* Quiet until it is worth promoting: the prototype's dismiss treatment
          for the option nobody needs yet, a real button once they might. Both
          press the same handler — only the weight changes (JQ-72). */}
      {promoted ? (
        <Button
          type="button"
          variant="default"
          className="notify-me__action"
          onClick={press}
          disabled={disabled || busy}
        >
          {busy ? NOTIFY_VERIFYING : label}
        </Button>
      ) : (
        <Link
          variant="quiet"
          className="notify-me__action"
          onClick={press}
          disabled={disabled || busy}
        >
          {busy ? NOTIFY_VERIFYING : label}
        </Link>
      )}
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
