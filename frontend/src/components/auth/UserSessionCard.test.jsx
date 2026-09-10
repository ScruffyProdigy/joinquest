import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import UserSessionCard from './UserSessionCard'
import { AuthProvider } from './AuthProvider'
import * as auth from '../../lib/auth'

vi.mock('../../lib/auth', () => ({
  logout: vi.fn(),
  fetchCurrentUser: vi.fn(),
}))

describe('UserSessionCard', () => {
  const user = {
    id: 'user-1',
    email: 'player@example.com',
    displayName: 'player',
    avatarKey: 'compass',
    avatarUrl: '/avatars/compass.png',
    createdAt: '2026-01-01T00:00:00Z',
  }

  beforeEach(() => {
    vi.mocked(auth.logout).mockReset()
    vi.mocked(auth.logout).mockResolvedValue(true)
    vi.mocked(auth.fetchCurrentUser).mockResolvedValue(user)
  })

  function renderSessionCard() {
    return render(
      <AuthProvider>
        <UserSessionCard user={user} />
      </AuthProvider>,
    )
  }

  it('shows the signed-in user', async () => {
    renderSessionCard()

    expect(await screen.findByText('Welcome back')).toBeInTheDocument()
    expect(screen.getByText('player@example.com')).toBeInTheDocument()
    expect(screen.getByText('player')).toBeInTheDocument()
  })

  it('shows change display for completed profiles', async () => {
    renderSessionCard()

    expect(await screen.findByRole('button', { name: 'Change display' })).toBeInTheDocument()
  })

  // Picking a face is one act, so it gets one component wherever it is offered.
  // The editor's icon list used to be a single column of its own while the
  // identity gate's two grids sat elsewhere, all drawn by hand.
  it('offers the journey icons on the shared two-up avatar grid', async () => {
    const person = userEvent.setup()
    renderSessionCard()

    await person.click(await screen.findByRole('button', { name: 'Change display' }))

    const list = await screen.findByRole('list')
    expect(list).toHaveClass('sm:grid-cols-2')
    expect(list).toHaveClass('list-none')
    expect(list).toHaveClass('p-0')
    expect(list).toHaveClass('m-0')
  })

  it('prompts new players to set up display', async () => {
    const newUser = {
      ...user,
      avatarKey: null,
      avatarUrl: null,
      displayName: 'player (new)',
    }

    render(
      <AuthProvider>
        <UserSessionCard user={newUser} />
      </AuthProvider>,
    )

    expect(await screen.findByRole('heading', { name: 'Set up your display' })).toBeInTheDocument()
    expect(screen.getByLabelText('Display name')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save display' })).toBeInTheDocument()
  })

  it('hides profile actions when showProfileActions is false', async () => {
    render(
      <AuthProvider>
        <UserSessionCard user={user} showProfileActions={false} />
      </AuthProvider>,
    )

    expect(await screen.findByText('Welcome back')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Change display' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Find my spirit animal' })).not.toBeInTheDocument()
  })

  it('logs out and clears the session', async () => {
    renderSessionCard()
    await screen.findByText('Welcome back')

    await userEvent.click(screen.getByRole('button', { name: 'Log out' }))

    expect(auth.logout).toHaveBeenCalledTimes(1)
  })
})
