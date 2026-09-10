import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import NotifyMeControl from './NotifyMeControl'
import * as push from '../../lib/push'
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
    expect(await screen.findByRole('button', { name: 'Notify me instead' })).toBeInTheDocument()
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

    const button = await screen.findByRole('button', { name: 'Notify me instead' })
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

      expect(screen.getByRole('button', { name: 'Notify me instead' })).toBeInTheDocument()
      expect(screen.queryByText('Leave this page and keep your spot.')).not.toBeInTheDocument()

      await act(async () => {
        vi.advanceTimersByTime(20000)
      })

      expect(screen.getByText('Leave this page and keep your spot.')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Notify me instead' })).toBeInTheDocument()
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

    expect(await screen.findByRole('button', { name: 'Notify me instead' })).toBeInTheDocument()
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
    expect(await screen.findByRole('button', { name: 'Notify me instead' })).toBeInTheDocument()
  })

  it('reports a failure instead of silently doing nothing', async () => {
    const user = userEvent.setup()
    vi.spyOn(push, 'enablePushNotifications').mockRejectedValue(new Error('network'))
    render(<NotifyMeControl />)

    await user.click(await screen.findByRole('button', { name: 'Notify me instead' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Notifications could not be switched on. Try again.',
    )
  })
})
