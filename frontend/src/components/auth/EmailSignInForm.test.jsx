import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import EmailSignInForm from './EmailSignInForm'
import { AuthProvider, useAuth } from './AuthProvider'
import * as auth from '../../lib/auth'

vi.mock('../../lib/auth', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    requestSignIn: vi.fn(),
    completeSignInWithCode: vi.fn(),
    fetchCurrentUser: vi.fn().mockResolvedValue(null),
  }
})

vi.mock('../../lib/authBroadcast', () => ({
  onAuthComplete: vi.fn(() => () => {}),
  notifyAuthComplete: vi.fn(),
}))

function SessionHint() {
  const { user } = useAuth()
  return user ? <p>Signed in as {user.email}</p> : null
}

function renderEmailSignInForm() {
  return render(
    <AuthProvider>
      <EmailSignInForm />
      <SessionHint />
    </AuthProvider>,
  )
}

describe('EmailSignInForm', () => {
  beforeEach(() => {
    vi.mocked(auth.requestSignIn).mockReset()
    vi.mocked(auth.requestSignIn).mockResolvedValue(true)
    vi.mocked(auth.completeSignInWithCode).mockReset()
    vi.mocked(auth.fetchCurrentUser).mockReset()
    vi.mocked(auth.fetchCurrentUser).mockResolvedValue(null)
  })

  it('shows the code field while the sign-in email is sending', async () => {
    let resolveRequest
    vi.mocked(auth.requestSignIn).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveRequest = () => resolve(true)
        }),
    )

    renderEmailSignInForm()
    await waitFor(() => {
      expect(screen.getByLabelText('Email')).toBeInTheDocument()
    })

    await userEvent.type(screen.getByLabelText('Email'), 'player@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Continue with email' }))

    expect(screen.getByRole('heading', { name: 'Enter your code' })).toBeInTheDocument()
    expect(screen.getByText('Sending sign-in email…')).toBeInTheDocument()
    expect(screen.getByLabelText('Sign-in code')).not.toBeDisabled()

    resolveRequest()
    await waitFor(() => {
      expect(screen.getByText(/check spam/i)).toBeInTheDocument()
    })
  })

  it('submits a sign-in email request and shows the code step', async () => {
    renderEmailSignInForm()
    await waitFor(() => {
      expect(screen.getByLabelText('Email')).toBeInTheDocument()
    })

    await userEvent.type(screen.getByLabelText('Email'), 'player@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Continue with email' }))

    await waitFor(() => {
      expect(auth.requestSignIn).toHaveBeenCalledWith('player@example.com')
      expect(screen.getByRole('heading', { name: 'Enter your code' })).toBeInTheDocument()
    })
    expect(screen.getByText(/check spam/i)).toBeInTheDocument()
    expect(screen.getByLabelText('Sign-in code')).not.toBeDisabled()
  })

  it('shows an error when the sign-in email request fails', async () => {
    vi.mocked(auth.requestSignIn).mockRejectedValue(new Error('SMTP unavailable'))

    renderEmailSignInForm()
    await userEvent.type(screen.getByLabelText('Email'), 'player@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Continue with email' }))

    expect(await screen.findByText('SMTP unavailable')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toBeInTheDocument()
  })

  it('submits a 6-digit code to complete sign-in', async () => {
    vi.mocked(auth.completeSignInWithCode).mockResolvedValue({
      id: 'user-1',
      email: 'player@example.com',
    })
    vi.mocked(auth.fetchCurrentUser).mockResolvedValue({
      id: 'user-1',
      email: 'player@example.com',
    })

    renderEmailSignInForm()
    await userEvent.type(screen.getByLabelText('Email'), 'player@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Continue with email' }))

    const codeInput = await screen.findByLabelText('Sign-in code')
    expect(codeInput).not.toBeDisabled()
    await userEvent.type(codeInput, '123456')

    await waitFor(() => {
      expect(auth.completeSignInWithCode).toHaveBeenCalledWith('player@example.com', '123456')
      expect(screen.getByText('Signed in as player@example.com')).toBeInTheDocument()
    })
  })

  it('shows a friendly error when the sign-in code is invalid', async () => {
    vi.mocked(auth.completeSignInWithCode).mockRejectedValue(
      new Error('Invalid or expired code. Try again or use the sign-in link in your email.'),
    )

    renderEmailSignInForm()
    await userEvent.type(screen.getByLabelText('Email'), 'player@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Continue with email' }))

    const codeInput = await screen.findByLabelText('Sign-in code')
    expect(codeInput).not.toBeDisabled()
    await userEvent.type(codeInput, '123456')

    expect(
      await screen.findByText(
        'Invalid or expired code. Try again or use the sign-in link in your email.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue' })).toBeInTheDocument()
  })
})
