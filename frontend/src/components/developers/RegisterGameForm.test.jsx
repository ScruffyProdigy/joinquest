import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth/AuthProvider'
import RegisterGameForm from './RegisterGameForm'
import { DEVELOPER_DRAFT_KEY, readRegistrationDraft, saveRegistrationDraft } from '../../lib/developerDraft'
import { registerMyGame } from '../../lib/developers'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../../test/setup'

vi.mock('../../lib/developers', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    registerMyGame: vi.fn(),
  }
})

vi.mock('../../lib/usePathname', async (importOriginal) => {
  const actual = await importOriginal()
  return { ...actual, navigateTo: vi.fn() }
})

const guestUser = {
  id: 'guest-1',
  email: null,
  displayName: 'guest#123456',
  isGuest: true,
  createdAt: '2026-01-01T00:00:00Z',
}

function renderForm(props = {}) {
  return render(
    <AuthProvider>
      <RegisterGameForm {...props} />
    </AuthProvider>,
  )
}

/** Wait for AuthProvider to settle so assertions see the real session state. */
async function formReady() {
  await waitFor(() => {
    expect(screen.getByLabelText(/game name/i)).toBeInTheDocument()
  })
}

async function fillRequiredFields(user) {
  await user.type(screen.getByLabelText(/game name/i), 'Word Hunt')
  await user.type(screen.getByLabelText(/short description/i), 'Find the words fastest.')
  await user.type(screen.getByLabelText(/api base url/i), 'https://api.wordhunt.example.com')
  await user.type(screen.getByLabelText(/contact email/i), 'dev@example.com')
}

describe('RegisterGameForm', () => {
  beforeEach(() => {
    sessionStorage.clear()
    vi.mocked(registerMyGame).mockReset()
  })

  afterEach(() => {
    sessionStorage.clear()
  })

  it('renders the form for a signed-out visitor instead of a sign-in prompt', async () => {
    mockUnauthenticatedSession()
    renderForm()

    await formReady()

    expect(screen.getByLabelText(/api base url/i)).toBeEnabled()
    expect(screen.queryByText(/sign in above to register your game/i)).not.toBeInTheDocument()
  })

  it('does not attempt registration when a signed-out visitor submits', async () => {
    mockUnauthenticatedSession()
    const onAccountRequired = vi.fn()
    const user = userEvent.setup()
    renderForm({ onAccountRequired })

    await formReady()
    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: /register game/i }))

    expect(onAccountRequired).toHaveBeenCalledTimes(1)
    expect(registerMyGame).not.toHaveBeenCalled()
  })

  it('does not attempt registration when a guest submits', async () => {
    mockAuthenticatedSession(guestUser)
    const onAccountRequired = vi.fn()
    const user = userEvent.setup()
    renderForm({ onAccountRequired })

    await formReady()
    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: /register game/i }))

    expect(onAccountRequired).toHaveBeenCalledTimes(1)
    expect(registerMyGame).not.toHaveBeenCalled()
  })

  it('saves entered values as a draft when a guest is sent to sign up', async () => {
    mockUnauthenticatedSession()
    const user = userEvent.setup()
    renderForm({ onAccountRequired: vi.fn() })

    await formReady()
    await fillRequiredFields(user)
    await user.click(screen.getByRole('button', { name: /register game/i }))

    expect(readRegistrationDraft()).toMatchObject({
      name: 'Word Hunt',
      apiBaseUrl: 'https://api.wordhunt.example.com',
      contactEmail: 'dev@example.com',
    })
  })

  it('restores a saved draft into the form fields', async () => {
    mockUnauthenticatedSession()
    saveRegistrationDraft({
      name: 'Word Hunt',
      slug: 'word-hunt',
      shortDescription: 'Find the words fastest.',
      apiBaseUrl: 'https://api.wordhunt.example.com',
      contactEmail: 'dev@example.com',
      websiteUrl: '',
      communityUrl: '',
    })

    renderForm()

    await formReady()

    expect(screen.getByLabelText(/game name/i)).toHaveValue('Word Hunt')
    expect(screen.getByLabelText(/^slug$/i)).toHaveValue('word-hunt')
    expect(screen.getByLabelText(/api base url/i)).toHaveValue('https://api.wordhunt.example.com')
    expect(screen.getByLabelText(/contact email/i)).toHaveValue('dev@example.com')
  })

  it('registers the game and clears the draft for a signed-in account', async () => {
    mockAuthenticatedSession()
    vi.mocked(registerMyGame).mockResolvedValue({
      game: { id: 'game-1' },
      connected: true,
      connectError: null,
    })
    const user = userEvent.setup()
    renderForm({ onAccountRequired: vi.fn() })

    await formReady()
    await user.type(screen.getByLabelText(/game name/i), 'Word Hunt')
    await user.type(screen.getByLabelText(/short description/i), 'Find the words fastest.')
    await user.type(screen.getByLabelText(/api base url/i), 'https://api.wordhunt.example.com')
    await user.click(screen.getByRole('button', { name: /register game/i }))

    await waitFor(() => {
      expect(registerMyGame).toHaveBeenCalledTimes(1)
    })
    expect(sessionStorage.getItem(DEVELOPER_DRAFT_KEY)).toBeNull()
  })
})
