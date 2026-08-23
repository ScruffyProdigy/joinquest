import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth/AuthProvider'
import DeveloperLandingPage from './DeveloperLandingPage'
import * as auth from '../../lib/auth'

vi.mock('../../lib/auth', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchCurrentUser: vi.fn(),
    requestSignIn: vi.fn(),
    completeSignInWithCode: vi.fn(),
  }
})

vi.mock('../../lib/developers', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchMyDeveloperApiKeys: vi.fn().mockResolvedValue([]),
  }
})

const signedInUser = {
  id: 'user-1',
  email: 'dev@example.com',
  displayName: 'dev',
  isGuest: false,
  createdAt: '2026-01-01T00:00:00Z',
}

describe('DeveloperLandingPage sign-in resume', () => {
  beforeEach(() => {
    sessionStorage.clear()
    window.history.replaceState(null, '', '/developers')
    vi.mocked(auth.fetchCurrentUser).mockResolvedValue(null)
    vi.mocked(auth.requestSignIn).mockResolvedValue(true)
    vi.mocked(auth.completeSignInWithCode).mockResolvedValue(signedInUser)
  })

  afterEach(() => {
    sessionStorage.clear()
    window.history.replaceState(null, '', '/')
  })

  it('drops the visitor into the AI wizard after signing in at the gate', async () => {
    const user = userEvent.setup()
    render(
      <AuthProvider>
        <DeveloperLandingPage />
      </AuthProvider>,
    )

    await waitFor(() => {
      expect(
        screen.getByRole('heading', { name: /have an idea for a multiplayer game/i }),
      ).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /connect an ai assistant/i }))
    expect(screen.getByRole('heading', { name: /build on joinquest/i })).toBeInTheDocument()

    // Sign in without leaving the page, the way the email-code flow does.
    vi.mocked(auth.fetchCurrentUser).mockResolvedValue(signedInUser)
    await user.type(screen.getByLabelText('Email'), 'dev@example.com')
    await user.click(screen.getByRole('button', { name: /continue with email/i }))
    await user.type(await screen.findByLabelText('Sign-in code'), '123456')

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /connect an ai assistant/i })).toBeInTheDocument()
    })
    expect(screen.queryByRole('heading', { name: /build on joinquest/i })).not.toBeInTheDocument()
  })
})
