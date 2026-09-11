import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import NotifyMeControl from './NotifyMeControl'
import * as push from '../../lib/push'
import { NOTIFY_ME } from '../../lib/playerCopy'
import {
  makePushSubscription,
  mockAuthenticatedSession,
  mockPushSupported,
  mockPushUnsupported,
} from '../../test/setup'

const CONFIGURED = {
  reachable: false,
  subscriptionCount: 0,
  publicKey: 'BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM',
}

function setUserAgent(value, maxTouchPoints = 0) {
  Object.defineProperty(navigator, 'userAgent', { writable: true, configurable: true, value })
  Object.defineProperty(navigator, 'maxTouchPoints', {
    writable: true,
    configurable: true,
    value: maxTouchPoints,
  })
}

const DESKTOP_UA = 'Mozilla/5.0 (X11; Linux x86_64) Chrome/120.0'
const IPHONE_UA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Safari/605.1'

describe('NotifyMeControl', () => {
  beforeEach(() => {
    setUserAgent(DESKTOP_UA)
    window.navigator.standalone = undefined
    mockPushSupported()
    mockAuthenticatedSession()
    try {
      window.localStorage.clear()
    } catch {
      // Ignored.
    }
  })

  afterEach(() => {
    vi.restoreAllMocks()
    mockPushUnsupported()
  })

  it('offers the control from the moment the player joins, before any timer', async () => {
    render(<NotifyMeControl />)

    // Someone who pockets their phone at 8s must already have been offered
    // something, so this cannot wait for the promote timer.
    expect(await screen.findByRole('button', { name: NOTIFY_ME })).toBeInTheDocument()
  })

  it('shows nothing when the deployment cannot send', async () => {
    mockAuthenticatedSession(undefined, {
      pushCapability: { reachable: false, subscriptionCount: 0, publicKey: null },
    })
    const { container } = render(<NotifyMeControl />)

    await waitFor(() => expect(container).toBeEmptyDOMElement())
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('spends the permission prompt only when pressed', async () => {
    const user = userEvent.setup()
    const enable = vi.spyOn(push, 'enablePushNotifications').mockResolvedValue({
      ...CONFIGURED,
      reachable: true,
      subscriptionCount: 1,
    })
    render(<NotifyMeControl />)

    const button = await screen.findByRole('button', { name: NOTIFY_ME })
    expect(enable).not.toHaveBeenCalled()

    await user.click(button)

    expect(enable).toHaveBeenCalledTimes(1)
    expect(await screen.findByText("You'll be notified")).toBeInTheDocument()
  })

  it('promotes the control after the timer without changing what it does', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    try {
      render(<NotifyMeControl />)
      // Flush the capability fetch before touching the clock, so its state
      // update lands inside act rather than racing the timer.
      await act(async () => {})

      expect(screen.getByRole('button', { name: NOTIFY_ME })).toBeInTheDocument()
      expect(screen.queryByText('Leave this page and keep your spot.')).not.toBeInTheDocument()

      await act(async () => {
        vi.advanceTimersByTime(20000)
      })

      expect(screen.getByText('Leave this page and keep your spot.')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: NOTIFY_ME })).toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('teaches the Home Screen gesture on iOS Safari instead of prompting', async () => {
    const user = userEvent.setup()
    setUserAgent(IPHONE_UA, 5)
    const enable = vi.spyOn(push, 'enablePushNotifications')
    render(<NotifyMeControl />)

    const button = await screen.findByRole('button', { name: 'Get notified on your phone' })
    await user.click(button)

    // iOS cannot deliver push to an uninstalled tab, so prompting would spend a
    // sticky permission on something that could not work.
    expect(enable).not.toHaveBeenCalled()
    expect(await screen.findByText('Tap Share')).toBeInTheDocument()
    expect(screen.getByText('in the bar at the bottom of the screen')).toBeInTheDocument()
    expect(screen.getByText('Choose Add to Home Screen')).toBeInTheDocument()
  })

  it('does not ask an installed iOS app to install', async () => {
    setUserAgent(IPHONE_UA, 5)
    window.navigator.standalone = true
    render(<NotifyMeControl />)

    expect(await screen.findByRole('button', { name: NOTIFY_ME })).toBeInTheDocument()
  })

  it('stops re-prompting once the player has dismissed the instructions', async () => {
    const user = userEvent.setup()
    setUserAgent(IPHONE_UA, 5)
    render(<NotifyMeControl />)

    await user.click(await screen.findByRole('button', { name: 'Get notified on your phone' }))
    await user.click(screen.getByRole('button', { name: 'Not now' }))

    await waitFor(() => expect(screen.queryByText('Tap Share')).not.toBeInTheDocument())

    // Dismissing means "not now", not "never" -- but it must not reappear on
    // the next press in the same week.
    await user.click(screen.getByRole('button', { name: 'Get notified on your phone' }))
    expect(screen.queryByText('Tap Share')).not.toBeInTheDocument()
  })

  it('explains a sticky denial rather than offering a button that cannot work', async () => {
    mockPushSupported({ permission: 'denied' })
    render(<NotifyMeControl />)

    expect(
      await screen.findByText(
        'Notifications are switched off for JoinQuest in your browser settings.',
      ),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('lets a subscribed player turn notifications back off', async () => {
    const user = userEvent.setup()
    mockAuthenticatedSession(undefined, {
      pushCapability: { ...CONFIGURED, reachable: true, subscriptionCount: 1 },
    })
    mockPushSupported({ existingSubscription: makePushSubscription() })
    const disable = vi.spyOn(push, 'disablePushNotifications').mockResolvedValue({
      ...CONFIGURED,
      reachable: false,
      subscriptionCount: 0,
    })
    render(<NotifyMeControl />)

    await user.click(await screen.findByRole('button', { name: 'Turn off notifications' }))

    expect(disable).toHaveBeenCalledTimes(1)
    expect(await screen.findByRole('button', { name: NOTIFY_ME })).toBeInTheDocument()
  })

  it('reports a failure instead of silently doing nothing', async () => {
    const user = userEvent.setup()
    vi.spyOn(push, 'enablePushNotifications').mockRejectedValue(new Error('network'))
    render(<NotifyMeControl />)

    await user.click(await screen.findByRole('button', { name: NOTIFY_ME }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Notifications could not be switched on. Try again.',
    )
  })

  it('never spends the permission prompt on a short queue', async () => {
    const enable = vi.spyOn(push, 'enablePushNotifications')
    const { container } = render(<NotifyMeControl estimatedWaitSeconds={15} />)

    // A denial is permanent per origin, and a fifteen-second wait is over
    // before leaving the page is worth doing.
    await waitFor(() => expect(container).toBeEmptyDOMElement())
    expect(enable).not.toHaveBeenCalled()
  })

  it('offers the control on a long queue', async () => {
    render(<NotifyMeControl estimatedWaitSeconds={240} />)

    expect(await screen.findByRole('button', { name: NOTIFY_ME })).toBeInTheDocument()
  })

  it('offers the control when the wait is not yet known', async () => {
    // Null is JQ-58's answer on a cold queue -- exactly where waits run long.
    render(<NotifyMeControl estimatedWaitSeconds={null} />)

    expect(await screen.findByRole('button', { name: NOTIFY_ME })).toBeInTheDocument()
  })

  it('says plainly that leaving still costs the spot when verification fails', async () => {
    const user = userEvent.setup()
    vi.spyOn(push, 'enablePushNotifications').mockResolvedValue({
      blocked: 'unverified',
      capability: { ...CONFIGURED, subscriptionCount: 1 },
    })
    render(<NotifyMeControl />)

    await user.click(await screen.findByRole('button', { name: NOTIFY_ME }))

    // Never a silent downgrade: the player pressed this believing it covered
    // them, so the retraction has to be as loud as the offer was.
    expect(await screen.findByRole('alert')).toHaveTextContent(
      "We couldn't reach your browser with a test notification.",
    )
    expect(
      screen.getByText('Leaving this page will still give up your spot. Stay here, or try again.'),
    ).toBeInTheDocument()
    expect(screen.queryByText("You'll be notified")).not.toBeInTheDocument()
  })

  it('lets the player retry after a failed verification', async () => {
    const user = userEvent.setup()
    const enable = vi
      .spyOn(push, 'enablePushNotifications')
      .mockResolvedValueOnce({ blocked: 'unverified', capability: CONFIGURED })
      .mockResolvedValueOnce({ ...CONFIGURED, reachable: true, subscriptionCount: 1 })
    render(<NotifyMeControl />)

    await user.click(await screen.findByRole('button', { name: NOTIFY_ME }))
    await user.click(await screen.findByRole('button', { name: 'Try again' }))

    expect(enable).toHaveBeenCalledTimes(2)
    expect(await screen.findByText("You'll be notified")).toBeInTheDocument()
  })

  it('tells the waiting page when the opt-in is in force', async () => {
    const onReachableChange = vi.fn()
    mockAuthenticatedSession(undefined, {
      pushCapability: { ...CONFIGURED, reachable: true, subscriptionCount: 1 },
    })
    render(<NotifyMeControl onReachableChange={onReachableChange} />)

    await waitFor(() => expect(onReachableChange).toHaveBeenCalledWith(true))
  })
})
