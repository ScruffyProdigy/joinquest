import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect } from 'vitest'
import AccountChip from './AccountChip'
import { AuthProvider } from '../auth/AuthProvider'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../../test/setup'

const GUEST = {
  id: 'guest-1',
  email: null,
  displayName: 'FrostFox2841',
  avatarKey: 'sigil-canine-frost',
  avatarUrl: '/avatars/sigils/canine-frost.svg',
  isGuest: true,
  createdAt: '2026-01-01T00:00:00Z',
}

function renderChip() {
  return render(
    <AuthProvider>
      <AccountChip />
    </AuthProvider>,
  )
}

describe('AccountChip', () => {
  it('offers sign-in to a visitor with no session', async () => {
    mockUnauthenticatedSession()
    renderChip()

    expect(await screen.findByRole('button', { name: 'Sign in or Join' })).toBeInTheDocument()
  })

  it('offers sign-in to a guest who already picked a name and avatar', async () => {
    mockAuthenticatedSession(GUEST)
    renderChip()

    expect(await screen.findByRole('button', { name: 'Sign in or Join' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /FrostFox2841/ })).not.toBeInTheDocument()
  })

  it('opens the sign-in dialog from the pill', async () => {
    const user = userEvent.setup()
    mockAuthenticatedSession(GUEST)
    renderChip()

    await user.click(await screen.findByRole('button', { name: 'Sign in or Join' }))

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('Sign in or create an account')).toBeInTheDocument()
    // The visitor already has a guest session, so "Play as guest" would be a no-op.
    expect(screen.queryByRole('button', { name: 'Play as guest' })).not.toBeInTheDocument()
  })

  it('links a signed-in player to their account via their avatar', async () => {
    mockAuthenticatedSession()
    renderChip()

    const link = await screen.findByRole('link', { name: /player/ })
    expect(link).toHaveAttribute('href', '/account')
    expect(screen.queryByRole('button', { name: 'Sign in or Join' })).not.toBeInTheDocument()
  })

  it('renders nothing until the session is known', async () => {
    mockAuthenticatedSession()
    const { container } = renderChip()

    expect(container).toBeEmptyDOMElement()
    await waitFor(() => {
      expect(container).not.toBeEmptyDOMElement()
    })
  })
})
