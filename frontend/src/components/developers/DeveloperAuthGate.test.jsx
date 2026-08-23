import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth/AuthProvider'
import DeveloperAuthGate from './DeveloperAuthGate'
import { JUMP_IN } from '../../lib/playerCopy'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../../test/setup'
import * as oauth from '../../lib/oauth'

vi.mock('../../lib/oauth', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchEnabledOAuthProviders: vi.fn().mockResolvedValue([]),
    startOAuthSignIn: vi.fn(),
  }
})

const guestUser = {
  id: 'guest-1',
  email: null,
  displayName: 'guest#123456',
  isGuest: true,
  createdAt: '2026-01-01T00:00:00Z',
}

function renderGate(props = {}) {
  return render(
    <AuthProvider>
      <DeveloperAuthGate onBack={() => {}} {...props} />
    </AuthProvider>,
  )
}

describe('DeveloperAuthGate', () => {
  beforeEach(() => {
    window.history.replaceState(null, '', '/developers')
  })

  it('offers sign-up without a guest option to a signed-out visitor', async () => {
    mockUnauthenticatedSession()
    renderGate()

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /build on joinquest/i })).toBeInTheDocument()
    })
    expect(screen.getByText(/a free account is required/i)).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: JUMP_IN })).not.toBeInTheDocument()
  })

  it('sends a guest to account settings to add an email', async () => {
    mockAuthenticatedSession(guestUser)
    renderGate()

    await waitFor(() => {
      expect(screen.getByText(/playing as a guest/i)).toBeInTheDocument()
    })
    expect(screen.getByRole('link', { name: /add an email to your account/i })).toHaveAttribute(
      'href',
      '/account',
    )
    expect(screen.queryByLabelText('Email')).not.toBeInTheDocument()
  })

  it('calls onBack when the back control is used', async () => {
    mockUnauthenticatedSession()
    const onBack = vi.fn()
    const user = userEvent.setup()
    renderGate({ onBack })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /build on joinquest/i })).toBeInTheDocument()
    })
    await user.click(screen.getByRole('button', { name: /back/i }))

    expect(onBack).toHaveBeenCalledTimes(1)
  })
})

describe('DeveloperAuthGate OAuth destination', () => {
  beforeEach(() => {
    window.history.replaceState(null, '', '/developers')
    mockUnauthenticatedSession()
    vi.mocked(oauth.fetchEnabledOAuthProviders).mockResolvedValue(['GOOGLE'])
  })

  it('sends the caller-supplied next key with the social sign-in', async () => {
    const user = userEvent.setup()
    renderGate({ next: 'dev-manual' })

    const googleButton = await screen.findByRole('button', { name: /continue with google/i })
    await user.click(googleButton)

    expect(oauth.startOAuthSignIn).toHaveBeenCalledWith('GOOGLE', 'dev-manual')
  })
})
