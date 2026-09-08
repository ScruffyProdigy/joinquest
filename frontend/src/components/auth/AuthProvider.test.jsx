import { act, render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { AuthProvider, useAuth } from './AuthProvider'
import { mockAuthenticatedSession } from '../../test/setup'

const SIGNED_IN_USER = {
  id: 'user-1',
  email: 'player@example.com',
  displayName: 'player',
  avatarKey: 'compass',
  avatarUrl: '/avatars/compass.png',
  avatarSource: 'STARTER',
  isGuest: false,
  createdAt: '2026-01-01T00:00:00Z',
}

function AuthConsumer() {
  const { user, loading } = useAuth()

  if (loading) {
    return <p>Loading</p>
  }

  return <p>{user ? `Signed in as ${user.email}` : 'Signed out'}</p>
}

describe('AuthProvider', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
  })

  it('shares the current user with useAuth', async () => {
    mockAuthenticatedSession()

    render(
      <AuthProvider>
        <AuthConsumer />
      </AuthProvider>,
    )

    await waitFor(() => {
      expect(screen.getByText('Signed in as player@example.com')).toBeInTheDocument()
    })
  })

  // JQ-205: a session result that was already in flight when the user logged
  // out used to re-seat them, leaving the page half signed in until a reload.
  describe('superseded session results', () => {
    let auth

    function AuthProbe() {
      auth = useAuth()
      return <p>{auth.user ? auth.user.email : 'Signed out'}</p>
    }

    async function renderSignedIn() {
      render(
        <AuthProvider>
          <AuthProbe />
        </AuthProvider>,
      )
      await screen.findByText('player@example.com')
    }

    it('ignores an accepted user issued under a cleared session', async () => {
      mockAuthenticatedSession()
      await renderSignedIn()

      const generation = auth.getSessionGeneration()
      act(() => {
        auth.clearSession()
      })
      await screen.findByText('Signed out')

      act(() => {
        auth.acceptSessionUser(SIGNED_IN_USER, { sessionGeneration: generation })
      })

      expect(screen.getByText('Signed out')).toBeInTheDocument()
    })

    it('still seats a user signing in after a logout', async () => {
      mockAuthenticatedSession()
      await renderSignedIn()

      act(() => {
        auth.clearSession()
      })
      await screen.findByText('Signed out')

      act(() => {
        auth.acceptSessionUser(SIGNED_IN_USER, {
          sessionGeneration: auth.getSessionGeneration(),
        })
      })

      await screen.findByText('player@example.com')
    })

    it('discards a refreshSession result that lands after a logout', async () => {
      mockAuthenticatedSession()
      const respond = global.fetch
      let releaseSession
      const sessionGate = new Promise((resolve) => {
        releaseSession = resolve
      })
      let gateArmed = false
      global.fetch = vi.fn(async (url, init) => {
        const query = JSON.parse(init?.body ?? '{}').query ?? ''
        if (gateArmed && /\bme\s*\{/.test(query)) {
          await sessionGate
        }
        return respond(url, init)
      })

      await renderSignedIn()

      gateArmed = true
      act(() => {
        void auth.refreshSession({ silent: true })
      })
      act(() => {
        auth.clearSession()
      })
      await screen.findByText('Signed out')

      await act(async () => {
        releaseSession()
        await new Promise((resolve) => setTimeout(resolve, 50))
      })

      expect(screen.getByText('Signed out')).toBeInTheDocument()
    })
  })
})
