import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import IdentityGate from './IdentityGate'
import { AuthProvider } from '../auth/AuthProvider'
import { SIGIL_FAMILIES } from '../../lib/guestIdentity'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../../test/setup'

const NAMELESS_GUEST = {
  id: 'guest-1',
  email: null,
  displayName: 'guest#123456',
  avatarKey: '',
  avatarUrl: '',
  avatarSource: null,
  isGuest: true,
  createdAt: '2026-01-01T00:00:00Z',
}

function renderGate() {
  return render(
    <AuthProvider>
      <IdentityGate />
    </AuthProvider>,
  )
}

/** The pickable avatar rows, which are the only buttons carrying a sigil image. */
function avatarChoices() {
  return screen
    .getAllByRole('button')
    .filter((button) => button.querySelector('img[src*="/avatars/sigil-"]'))
}

async function waitForGate() {
  await waitFor(() => {
    expect(screen.getByRole('heading', { name: 'Welcome to JoinQuest' })).toBeInTheDocument()
  })
}

describe('IdentityGate', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
  })

  it('prompts a signed-out visitor', async () => {
    mockUnauthenticatedSession()
    renderGate()

    await waitForGate()
    expect(screen.getByText('Select an avatar to start playing')).toBeInTheDocument()
  })

  it('prompts a guest who has a session but no name or avatar', async () => {
    mockAuthenticatedSession(NAMELESS_GUEST)
    renderGate()

    await waitForGate()
  })

  it('offers one pickable avatar per sigil, each with its own name', async () => {
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    const choices = avatarChoices()
    expect(choices).toHaveLength(SIGIL_FAMILIES.length)

    const sources = choices.map((button) => button.querySelector('img').getAttribute('src'))
    expect(new Set(sources).size).toBe(SIGIL_FAMILIES.length)

    for (const button of choices) {
      expect(button).toHaveTextContent(/^[A-Za-z]+\d{4}$/)
    }
  })

  it('says the name and avatar are for JoinQuest as a whole', async () => {
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    expect(
      screen.getByText('This is your name and avatar across all of JoinQuest, not just one game.'),
    ).toBeInTheDocument()
  })

  it('assigns the name and avatar, then dismisses onto whatever is behind it', async () => {
    const user = userEvent.setup()
    mockAuthenticatedSession(NAMELESS_GUEST)
    renderGate()
    await waitForGate()

    const choice = avatarChoices()[0]
    const chosenName = choice.textContent.trim()
    await user.click(choice)

    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: 'Welcome to JoinQuest' })).not.toBeInTheDocument()
    })

    const profileCall = global.fetch.mock.calls
      .map(([, init]) => JSON.parse(init.body))
      .find((body) => body.query.includes('updatePlayerProfile'))
    expect(profileCall.variables.displayName).toBe(chosenName)
    expect(profileCall.variables.avatarKey).toMatch(/^sigil-/)
  })

  it('creates a session first when the visitor has none', async () => {
    const user = userEvent.setup()
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    await user.click(avatarChoices()[0])

    await waitFor(() => {
      const queries = global.fetch.mock.calls.map(([, init]) => JSON.parse(init.body).query)
      expect(queries.some((query) => query.includes('createGuestSession'))).toBe(true)
    })
  })

  it('offers a sign-in path that is subordinate to the avatar picks', async () => {
    const user = userEvent.setup()
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    const signIn = screen.getByRole('button', { name: 'Log in or create account' })
    await user.click(signIn)

    // The sign-in view must not offer "Jump in", which would loop back to this gate.
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Back to avatars' })).toBeInTheDocument()
    })
    expect(screen.queryByRole('button', { name: 'Jump in' })).not.toBeInTheDocument()
  })

  it('cannot be dismissed without picking an avatar or signing in', async () => {
    // pointerEventsCheck is off because the open dialog blocks pointer events on
    // everything behind it — clicking the scrim is the only outside click available.
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    const dialog = screen.getByRole('dialog')
    expect(within(dialog).queryByRole('button', { name: /close/i })).not.toBeInTheDocument()

    await user.keyboard('{Escape}')
    expect(screen.getByRole('heading', { name: 'Welcome to JoinQuest' })).toBeInTheDocument()

    await user.click(document.querySelector('[data-slot="dialog-overlay"]'))
    expect(screen.getByRole('heading', { name: 'Welcome to JoinQuest' })).toBeInTheDocument()
  })

  it('never appears for a signed-in player who already has both', async () => {
    mockAuthenticatedSession()
    renderGate()

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled()
    })
    expect(screen.queryByRole('heading', { name: 'Welcome to JoinQuest' })).not.toBeInTheDocument()
  })

  it('never re-prompts a returning guest who already picked', async () => {
    mockAuthenticatedSession({
      ...NAMELESS_GUEST,
      displayName: 'FrostFox4827',
      avatarKey: 'sigil-canine',
      avatarUrl: '/avatars/sigil-canine.svg',
    })
    renderGate()

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled()
    })
    expect(screen.queryByRole('heading', { name: 'Welcome to JoinQuest' })).not.toBeInTheDocument()
  })
})
