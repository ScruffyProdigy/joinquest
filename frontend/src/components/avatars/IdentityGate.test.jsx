import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import IdentityGate from './IdentityGate'
import { AuthProvider } from '../auth/AuthProvider'
import { GUEST_IDENTITY_CHOICES } from '../../lib/guestIdentity'
import {
  mockAuthenticatedSession,
  mockUnauthenticatedSession,
  waitForClickableDialog,
} from '../../test/setup'

const NAMELESS_GUEST = {
  id: 'guest-1',
  email: null,
  displayName: null,
  avatarKey: '',
  avatarUrl: '',
  avatarSource: null,
  isGuest: true,
  createdAt: '2026-01-01T00:00:00Z',
}

/** An account migrated by 000045: the spirit animal survived, the name did not. */
const NAMELESS_MEMBER = {
  id: 'user-2',
  email: 'ryan@example.com',
  displayName: null,
  avatarKey: null,
  avatarUrl: '/avatars/spirit/fox.svg',
  avatarSource: 'SPIRIT_ANIMAL',
  isGuest: false,
  createdAt: '2026-01-01T00:00:00Z',
}

/** The mirror case: a name on file and no face yet. */
const FACELESS_MEMBER = {
  id: 'user-3',
  email: 'ryan@example.com',
  displayName: 'Ryan',
  avatarKey: '',
  avatarUrl: '',
  avatarSource: null,
  isGuest: false,
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
    .filter((button) => button.querySelector('img[src*="/avatars/sigils/"]'))
}

async function waitForGate() {
  await waitForHeading('Welcome to JoinQuest')
}

/**
 * The gate is a modal dialog, so its heading lands in the DOM a render before
 * the dialog becomes clickable. Wait for both, or a click here races the gate.
 */
async function waitForHeading(name) {
  await waitFor(() => {
    expect(screen.getByRole('heading', { name })).toBeInTheDocument()
  })
  await waitForClickableDialog()
}

/** The variables of the updatePlayerProfile mutation, once one has been sent. */
function profileCallVariables() {
  return global.fetch.mock.calls
    .map(([, init]) => JSON.parse(init.body))
    .find((body) => body.query.includes('updatePlayerProfile'))?.variables
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
    expect(choices).toHaveLength(GUEST_IDENTITY_CHOICES)

    const sources = choices.map((button) => button.querySelector('img').getAttribute('src'))
    expect(new Set(sources).size).toBe(GUEST_IDENTITY_CHOICES)

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

  it('selects on the first tap and takes nothing until it is confirmed', async () => {
    const user = userEvent.setup()
    mockAuthenticatedSession(NAMELESS_GUEST)
    renderGate()
    await waitForGate()

    const choice = avatarChoices()[0]
    const chosenName = choice.textContent.trim()
    await user.click(choice)

    // Selected, named on the confirm button, and nothing sent yet.
    expect(choice).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: `Jump in as ${chosenName}` })).toBeInTheDocument()
    expect(profileCallVariables()).toBeUndefined()
  })

  it('moves the selection when a second avatar is tapped, rather than taking it', async () => {
    const user = userEvent.setup()
    mockAuthenticatedSession(NAMELESS_GUEST)
    renderGate()
    await waitForGate()

    await user.click(avatarChoices()[0])
    await user.click(avatarChoices()[1])

    expect(avatarChoices()[0]).toHaveAttribute('aria-pressed', 'false')
    expect(avatarChoices()[1]).toHaveAttribute('aria-pressed', 'true')
    expect(profileCallVariables()).toBeUndefined()
  })

  it('assigns the name and avatar, then dismisses onto whatever is behind it', async () => {
    const user = userEvent.setup()
    mockAuthenticatedSession(NAMELESS_GUEST)
    renderGate()
    await waitForGate()

    const choice = avatarChoices()[0]
    const chosenName = choice.textContent.trim()
    await user.click(choice)
    await user.click(screen.getByRole('button', { name: `Jump in as ${chosenName}` }))

    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: 'Welcome to JoinQuest' })).not.toBeInTheDocument()
    })

    const variables = profileCallVariables()
    expect(variables.displayName).toBe(chosenName)
    expect(variables.avatarKey).toMatch(/^sigil-/)
  })

  it('creates a session first when the visitor has none', async () => {
    const user = userEvent.setup()
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    // Tapping the selected row again is the other way to confirm it.
    await user.click(avatarChoices()[0])
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

    // The sign-in view must not offer "Play as guest", which would loop back to this gate.
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Back to avatars' })).toBeInTheDocument()
    })
    expect(screen.queryByRole('button', { name: 'Play as guest' })).not.toBeInTheDocument()
  })

  it('renders the sign-in action as an outlined pill, never as underlined text', async () => {
    // JQ-72: the prototype has no underlined link anywhere. Its secondary action
    // is a full-width outlined pill, and this overlay is where that was worst.
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    const signIn = screen.getByRole('button', { name: 'Log in or create account' })
    expect(signIn).toHaveClass('border-primary')
    expect(signIn).toHaveClass('rounded-[99px]')
    expect(signIn).not.toHaveClass('underline')
    expect(signIn).not.toHaveClass('hover:underline')
  })

  it('lays the choices out as a bare grid, with no list bullets', async () => {
    // Tailwind's theme and utilities load without preflight (see tailwind.css), so
    // a `ul` keeps the browser's disc marker and 40px indent unless it opts out.
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    const list = screen.getByRole('list')
    expect(list).toHaveClass('list-none')
    expect(list).toHaveClass('p-0')
    expect(list).toHaveClass('m-0')
  })

  // The gate used to be a centred card sized to its content, which on a phone was
  // taller than the screen: the sign-in offer and everything under it fell off the
  // bottom, with the page behind showing through. A takeover cannot outgrow the
  // screen, and scrolls if the content ever does.
  it('fills the screen rather than floating a card over it', async () => {
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('data-takeover')
    expect(dialog).toHaveClass('inset-0')
    expect(dialog).toHaveClass('overflow-y-auto')
    expect(dialog).not.toHaveClass('rounded-lg')
  })

  it('offers the avatars two up, at every width', async () => {
    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    const list = screen.getByRole('list')
    expect(list).toHaveClass('grid-cols-2')
    // Not `sm:grid-cols-2`: one up below `sm` is what made six rows too tall.
    expect(list.className).not.toMatch(/sm:grid-cols/)
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
        avatarKey: 'sigil-canine-frost',
      avatarUrl: '/avatars/sigils/canine-frost.svg',
    })
    renderGate()

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalled()
    })
    expect(screen.queryByRole('heading', { name: 'Welcome to JoinQuest' })).not.toBeInTheDocument()
  })
})

describe('IdentityGate, when only the name is missing', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
  })

  it('asks for a name alone and leaves the avatar out of it', async () => {
    mockAuthenticatedSession(NAMELESS_MEMBER)
    renderGate()
    await waitForHeading('One more thing')

    expect(screen.getByLabelText('What should we call you?')).toBeInTheDocument()
    expect(screen.getByText('Your avatar stays as it is.')).toBeInTheDocument()
    expect(avatarChoices()).toHaveLength(0)
  })

  it('shows the avatar they already have rather than offering to replace it', async () => {
    mockAuthenticatedSession(NAMELESS_MEMBER)
    renderGate()
    await waitForHeading('One more thing')

    const dialog = screen.getByRole('dialog')
    expect(dialog.querySelector(`img[src="${NAMELESS_MEMBER.avatarUrl}"]`)).not.toBeNull()
  })

  it('does not present itself as a guest sign-in to an account holder', async () => {
    mockAuthenticatedSession(NAMELESS_MEMBER)
    renderGate()
    await waitForHeading('One more thing')

    expect(screen.queryByRole('heading', { name: 'Welcome to JoinQuest' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Log in or create account' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Play as guest' })).not.toBeInTheDocument()
  })

  it('saves the typed name without sending an avatar key', async () => {
    const user = userEvent.setup()
    mockAuthenticatedSession(NAMELESS_MEMBER)
    renderGate()
    await waitForHeading('One more thing')

    await user.type(screen.getByLabelText('What should we call you?'), 'Ryan')
    await user.click(screen.getByRole('button', { name: 'Save and continue' }))

    await waitFor(() => {
      expect(profileCallVariables()).toBeDefined()
    })
    // No avatarKey at all: the backend leaves the spirit animal untouched.
    expect(profileCallVariables()).toEqual({ displayName: 'Ryan' })
  })

  it('will not save an empty name', async () => {
    mockAuthenticatedSession(NAMELESS_MEMBER)
    renderGate()
    await waitForHeading('One more thing')

    expect(screen.getByRole('button', { name: 'Save and continue' })).toBeDisabled()
  })

  it('keeps the sign-in path for a guest, who has an account to gain', async () => {
    mockAuthenticatedSession({ ...NAMELESS_MEMBER, id: 'guest-2', email: null, isGuest: true })
    renderGate()
    await waitForHeading('One more thing')

    expect(screen.getByRole('button', { name: 'Log in or create account' })).toBeInTheDocument()
  })
})

describe('IdentityGate, when only the avatar is missing', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
  })

  it('offers faces alone, under the name they already chose', async () => {
    mockAuthenticatedSession(FACELESS_MEMBER)
    renderGate()
    await waitForHeading('Pick your face')

    expect(screen.getByText('Playing as Ryan.')).toBeInTheDocument()
    expect(avatarChoices()).toHaveLength(GUEST_IDENTITY_CHOICES)
    // No generated handles: their own name is not up for replacement.
    for (const button of avatarChoices()) {
      expect(button).not.toHaveTextContent(/\d{4}/)
    }
    expect(screen.queryByLabelText('What should we call you?')).not.toBeInTheDocument()
  })

  it('does not present itself as a guest sign-in to an account holder', async () => {
    mockAuthenticatedSession(FACELESS_MEMBER)
    renderGate()
    await waitForHeading('Pick your face')

    expect(screen.queryByRole('button', { name: 'Log in or create account' })).not.toBeInTheDocument()
  })

  it('saves the picked avatar against the name already on file', async () => {
    const user = userEvent.setup()
    mockAuthenticatedSession(FACELESS_MEMBER)
    renderGate()
    await waitForHeading('Pick your face')

    await user.click(avatarChoices()[0])

    await waitFor(() => {
      expect(profileCallVariables()).toBeDefined()
    })
    expect(profileCallVariables().displayName).toBe('Ryan')
    expect(profileCallVariables().avatarKey).toMatch(/^sigil-/)
  })

  it('names each face for anyone not reading by sight', async () => {
    mockAuthenticatedSession(FACELESS_MEMBER)
    renderGate()
    await waitForHeading('Pick your face')

    for (const button of avatarChoices()) {
      expect(button.getAttribute('aria-label')).toMatch(/^[A-Za-z]+ [A-Za-z]+$/)
    }
  })
  // The face-only prompt used to draw its own grid, and drifted to a six-across
  // row while the guest picker stayed two-up. Same six sigils, two screens --
  // which half of an identity was missing decided which one a player got.
  it('draws the faces in the grid the guest picker draws them in', async () => {
    mockAuthenticatedSession(FACELESS_MEMBER)
    const faceless = renderGate()
    await waitForHeading('Pick your face')
    const facesGrid = avatarChoices()[0].closest('ul').className
    faceless.unmount()

    mockUnauthenticatedSession()
    renderGate()
    await waitForGate()

    expect(avatarChoices()[0].closest('ul').className).toBe(facesGrid)
  })

  // `label` exists for exactly this row (see generateGuestIdentity): someone who
  // already has a name still has to be told which disc they are reaching for.
  it('names each face on the row itself, not only for a screen reader', async () => {
    mockAuthenticatedSession(FACELESS_MEMBER)
    renderGate()
    await waitForHeading('Pick your face')

    for (const button of avatarChoices()) {
      expect(button).toHaveTextContent(/^[A-Za-z]+ [A-Za-z]+$/)
    }
  })

  it('lays the faces out as a bare grid, with no list bullets', async () => {
    mockAuthenticatedSession(FACELESS_MEMBER)
    renderGate()
    await waitForHeading('Pick your face')

    const list = screen.getByRole('list')
    expect(list).toHaveClass('list-none')
    expect(list).toHaveClass('p-0')
    expect(list).toHaveClass('m-0')
  })
})
