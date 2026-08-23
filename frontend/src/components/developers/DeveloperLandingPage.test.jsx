import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth/AuthProvider'
import DeveloperLandingPage from './DeveloperLandingPage'
import { SIGN_IN_HEADING } from '../../lib/playerCopy'
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

vi.mock('../../lib/developers', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchMyDeveloperApiKeys: vi.fn().mockResolvedValue([]),
    registerMyGame: vi.fn(),
  }
})

const guestUser = {
  id: 'guest-1',
  email: null,
  displayName: 'guest#123456',
  isGuest: true,
  createdAt: '2026-01-01T00:00:00Z',
}

function renderLandingPage() {
  return render(
    <AuthProvider>
      <DeveloperLandingPage />
    </AuthProvider>,
  )
}

/** Wait for AuthProvider to settle so gating assertions see the real session state. */
async function pageReady() {
  await waitFor(() => {
    expect(
      screen.getByRole('heading', { name: /have an idea for a multiplayer game/i }),
    ).toBeInTheDocument()
  })
}

describe('DeveloperLandingPage', () => {
  beforeEach(() => {
    sessionStorage.clear()
    window.history.replaceState(null, '', '/developers')
  })

  afterEach(() => {
    sessionStorage.clear()
    window.history.replaceState(null, '', '/')
  })

  it('opens manual registration when ?path=manual is in the URL', () => {
    window.history.replaceState(null, '', '/developers?path=manual')

    renderLandingPage()

    expect(screen.getByRole('heading', { name: /register your game/i })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: /how do you want to get started/i })).not.toBeInTheDocument()
  })

  it('opens AI assistant setup when ?path=ai is in the URL', () => {
    window.history.replaceState(null, '', '/developers?path=ai')

    renderLandingPage()

    expect(screen.getByRole('heading', { name: /connect an ai assistant/i })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: /how do you want to get started/i })).not.toBeInTheDocument()
  })

  it('navigates to manual registration from route picker', async () => {
    const user = userEvent.setup()
    renderLandingPage()

    await user.click(screen.getByRole('button', { name: /register in the browser/i }))

    expect(screen.getByRole('heading', { name: /register your game/i })).toBeInTheDocument()
    expect(window.location.search).toBe('?path=manual')
  })

  it('does not put a sign-in card above the pitch for a signed-out visitor', async () => {
    mockUnauthenticatedSession()
    renderLandingPage()

    await pageReady()

    expect(screen.queryByRole('heading', { name: SIGN_IN_HEADING })).not.toBeInTheDocument()
  })

  it('prompts a signed-out visitor to sign up before the AI wizard', async () => {
    mockUnauthenticatedSession()
    const user = userEvent.setup()
    renderLandingPage()

    await pageReady()
    await user.click(screen.getByRole('button', { name: /connect an ai assistant/i }))

    expect(screen.getByRole('heading', { name: /build on joinquest/i })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: /connect an ai assistant/i })).not.toBeInTheDocument()
  })

  it('prompts a guest to sign up before the AI wizard', async () => {
    mockAuthenticatedSession(guestUser)
    const user = userEvent.setup()
    renderLandingPage()

    await pageReady()
    await user.click(screen.getByRole('button', { name: /connect an ai assistant/i }))

    expect(screen.getByRole('heading', { name: /build on joinquest/i })).toBeInTheDocument()
    expect(screen.getByText(/playing as a guest/i)).toBeInTheDocument()
  })

  it('opens the AI wizard directly for a signed-in account', async () => {
    mockAuthenticatedSession()
    const user = userEvent.setup()
    renderLandingPage()

    await pageReady()
    await user.click(screen.getByRole('button', { name: /connect an ai assistant/i }))

    expect(screen.getByRole('heading', { name: /connect an ai assistant/i })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: /build on joinquest/i })).not.toBeInTheDocument()
  })

  it('lets a signed-out visitor open and fill the manual form without signing in', async () => {
    mockUnauthenticatedSession()
    const user = userEvent.setup()
    renderLandingPage()

    await pageReady()
    await user.click(screen.getByRole('button', { name: /register in the browser/i }))
    await user.type(screen.getByLabelText(/game name/i), 'Word Hunt')

    expect(screen.getByLabelText(/game name/i)).toHaveValue('Word Hunt')
    expect(screen.queryByRole('heading', { name: /build on joinquest/i })).not.toBeInTheDocument()
  })

  it('prompts sign-up when a signed-out visitor submits the manual form', async () => {
    mockUnauthenticatedSession()
    const user = userEvent.setup()
    renderLandingPage()

    await pageReady()
    await user.click(screen.getByRole('button', { name: /register in the browser/i }))
    await user.type(screen.getByLabelText(/game name/i), 'Word Hunt')
    await user.type(screen.getByLabelText(/short description/i), 'Find the words fastest.')
    await user.type(screen.getByLabelText(/api base url/i), 'https://api.wordhunt.example.com')
    await user.type(screen.getByLabelText(/contact email/i), 'dev@example.com')
    await user.click(screen.getByRole('button', { name: /register game/i }))

    expect(screen.getByRole('heading', { name: /build on joinquest/i })).toBeInTheDocument()
  })

  it('returns the form with its values intact when leaving the sign-up prompt', async () => {
    mockUnauthenticatedSession()
    const user = userEvent.setup()
    renderLandingPage()

    await pageReady()
    await user.click(screen.getByRole('button', { name: /register in the browser/i }))
    await user.type(screen.getByLabelText(/game name/i), 'Word Hunt')
    await user.type(screen.getByLabelText(/short description/i), 'Find the words fastest.')
    await user.type(screen.getByLabelText(/api base url/i), 'https://api.wordhunt.example.com')
    await user.type(screen.getByLabelText(/contact email/i), 'dev@example.com')
    await user.click(screen.getByRole('button', { name: /register game/i }))
    await user.click(screen.getByRole('button', { name: /back/i }))

    expect(screen.getByLabelText(/game name/i)).toHaveValue('Word Hunt')
    expect(screen.getByLabelText(/api base url/i)).toHaveValue('https://api.wordhunt.example.com')
  })
})

describe('DeveloperLandingPage OAuth destination', () => {
  beforeEach(() => {
    sessionStorage.clear()
    window.history.replaceState(null, '', '/developers')
    mockUnauthenticatedSession()
    vi.mocked(oauth.fetchEnabledOAuthProviders).mockResolvedValue(['GOOGLE'])
    vi.mocked(oauth.startOAuthSignIn).mockReset()
  })

  afterEach(() => {
    sessionStorage.clear()
    window.history.replaceState(null, '', '/')
  })

  it('returns to the AI wizard after signing up from the AI gate', async () => {
    const user = userEvent.setup()
    renderLandingPage()

    await pageReady()
    await user.click(screen.getByRole('button', { name: /connect an ai assistant/i }))
    await user.click(await screen.findByRole('button', { name: /continue with google/i }))

    expect(oauth.startOAuthSignIn).toHaveBeenCalledWith('GOOGLE', 'dev-ai')
  })

  it('returns to the filled form after signing up from the register gate', async () => {
    const user = userEvent.setup()
    renderLandingPage()

    await pageReady()
    await user.click(screen.getByRole('button', { name: /register in the browser/i }))
    await user.type(screen.getByLabelText(/game name/i), 'Word Hunt')
    await user.type(screen.getByLabelText(/short description/i), 'Find the words fastest.')
    await user.type(screen.getByLabelText(/api base url/i), 'https://api.wordhunt.example.com')
    await user.type(screen.getByLabelText(/contact email/i), 'dev@example.com')
    await user.click(screen.getByRole('button', { name: /register game/i }))
    await user.click(await screen.findByRole('button', { name: /continue with google/i }))

    expect(oauth.startOAuthSignIn).toHaveBeenCalledWith('GOOGLE', 'dev-manual')
  })
})
