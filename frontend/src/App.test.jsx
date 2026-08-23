import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import App from './App'
import { IDENTITY_GATE_HEADING } from './lib/playerCopy'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from './test/setup'

describe('App Component', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        ...window.location,
        pathname: '/',
        search: '',
        assign: vi.fn(),
      },
    })
  })

  it('gates a signed-out visitor behind the avatar picker', async () => {
    mockUnauthenticatedSession()
    render(<App />)

    expect(screen.getByRole('heading', { level: 1, name: 'JoinQuest' })).toBeInTheDocument()
    expect(screen.getByText('Find your group. Play together.')).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: IDENTITY_GATE_HEADING })).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: 'Log in or create account' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Room' })).not.toBeInTheDocument()
  })

  it('shows the signed-in user when authenticated', async () => {
    mockAuthenticatedSession()
    render(<App />)

    expect(await screen.findByText('Welcome back')).toBeInTheDocument()
    expect(screen.getByText('player@example.com')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Log out' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Available games' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /get started for developers/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Create room' })).not.toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Rock Paper Scissors Lizard Robot' })).toBeInTheDocument()
  })

  it('renders the sign-in link completion page on /auth/complete', async () => {
    window.history.replaceState({}, '', '/auth/complete?token=test-token')
    const assign = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...window.location, assign, pathname: '/auth/complete', search: '?token=test-token' },
    })

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        data: {
          completeSignInWithLink: {
            id: 'user-1',
            email: 'player@example.com',
            displayName: 'player',
            createdAt: '2026-01-01T00:00:00Z',
          },
        },
      }),
    })

    render(<App />)

    expect(await screen.findByText('Signing you in')).toBeInTheDocument()
  })

  it('renders the OAuth error page on /auth/oauth/complete', async () => {
    window.history.replaceState({}, '', '/auth/oauth/complete?error=provider_error')
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        ...window.location,
        assign: vi.fn(),
        pathname: '/auth/oauth/complete',
        search: '?error=provider_error',
      },
    })

    mockUnauthenticatedSession()

    render(<App />)

    expect(await screen.findByText(/Could not sign in with that provider/i)).toBeInTheDocument()
    expect(screen.queryByText('Signing you in')).not.toBeInTheDocument()
  })

  it('shows legal links in the site footer', async () => {
    mockUnauthenticatedSession()
    render(<App />)

    expect(screen.getByRole('link', { name: 'Terms of Service' })).toHaveAttribute('href', '/terms')
    expect(screen.getByRole('link', { name: 'Privacy Policy' })).toHaveAttribute('href', '/privacy')
  })

  it('renders the terms of service page', async () => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        ...window.location,
        pathname: '/terms',
        search: '',
        assign: vi.fn(),
      },
    })
    window.history.replaceState({}, '', '/terms')
    mockUnauthenticatedSession()
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Terms of Service' })).toBeInTheDocument()
    expect(screen.getByText(/early-stage product/i)).toBeInTheDocument()
  })

  it('renders the privacy policy page', async () => {
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        ...window.location,
        pathname: '/privacy',
        search: '',
        assign: vi.fn(),
      },
    })
    window.history.replaceState({}, '', '/privacy')
    mockUnauthenticatedSession()
    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Privacy Policy' })).toBeInTheDocument()
    expect(screen.getByText(/do not sell your personal information/i)).toBeInTheDocument()
  })
})
