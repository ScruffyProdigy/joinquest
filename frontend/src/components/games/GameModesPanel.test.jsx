import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import GameModesPanel from './GameModesPanel'
import IdentityPromptProvider from '../avatars/IdentityPromptProvider'
import { joinQueue } from '../../lib/queue'

const authState = { user: null, loading: false, refreshSession: async () => {} }

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => authState,
}))

vi.mock('../avatars/IdentityGate', () => ({
  default: () => <div data-testid="identity-gate">Pick an avatar</div>,
}))

function identifiedUser() {
  return { id: 'user-1', isGuest: true, displayName: 'Ryan', avatarKey: 'sigil-canine' }
}

const refreshBeforeIdentity = vi.fn()
const refreshAfterIdentity = vi.fn()

vi.mock('../rooms/ActiveRoomProvider', () => ({
  // The real provider rebuilds `refresh` whenever the session changes, and the
  // session-less one clears the room instead of loading it. Handing back a
  // different function per session is what makes a stale closure detectable.
  useActiveRoom: () => ({
    refresh: authState.user ? refreshAfterIdentity : refreshBeforeIdentity,
    openRoom: vi.fn(),
  }),
}))

vi.mock('../../lib/tables', () => ({
  createPrivateTable: vi.fn(),
}))

const navigateTo = vi.fn()
vi.mock('../../lib/usePathname', () => ({
  navigateTo: (...args) => navigateTo(...args),
  usePathname: () => '/games/legendary-quest',
}))

vi.mock('../../lib/queue', () => ({
  joinQueue: vi.fn(),
  fetchMyQueueStatus: vi.fn(),
  leaveQueue: vi.fn(),
  prefetchSubscriptionAuth: vi.fn().mockResolvedValue('Bearer test'),
  subscribeToQueue: vi.fn().mockResolvedValue(() => {}),
}))

function baseGame(mode) {
  return {
    id: 'game-1',
    slug: 'legendary-quest',
    modes: [{ id: 'mode-1', modeKey: 'legendary', displayName: 'Legendary', status: 'active', queues: [], seats: [], queuePaths: [], ...mode }],
  }
}

describe('GameModesPanel locked mode', () => {
  it('renders queue actions when accessible', () => {
    render(<GameModesPanel game={baseGame({ eligibility: { accessible: true } })} />)
    expect(screen.queryByText(/Complete 50 Ranked matches/)).not.toBeInTheDocument()
  })

  it('renders reason and progress when locked with a leaf requirement', () => {
    render(
      <GameModesPanel
        game={baseGame({
          eligibility: {
            accessible: false,
            reason: 'Complete 50 Ranked matches to unlock.',
            unlockModeKey: null,
            requirement: { __typename: 'RequirementLeaf', label: 'Ranked matches', current: 12, target: 50 },
          },
        })}
      />,
    )
    expect(screen.getByText('Complete 50 Ranked matches to unlock.')).toBeInTheDocument()
    expect(screen.getByText('12/50 Ranked matches')).toBeInTheDocument()
  })

  it('renders a boolean gate with no progress readout', () => {
    render(
      <GameModesPanel
        game={baseGame({
          eligibility: { accessible: false, reason: 'Complete the tutorial to unlock.', unlockModeKey: null, requirement: null },
        })}
      />,
    )
    expect(screen.getByText('Complete the tutorial to unlock.')).toBeInTheDocument()
  })

  it('routes to unlockModeKey when the locked reason is clicked', async () => {
    const onNavigateToMode = vi.fn()
    render(
      <GameModesPanel
        game={baseGame({
          modeKey: 'standard',
          eligibility: {
            accessible: false,
            reason: 'You have no Standard-legal decks.',
            unlockModeKey: 'deck-builder',
            requirement: { __typename: 'RequirementLeaf', label: 'Standard-legal decks', current: 0, target: 1 },
          },
        })}
        onNavigateToMode={onNavigateToMode}
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: 'You have no Standard-legal decks.' }))
    expect(onNavigateToMode).toHaveBeenCalledWith('deck-builder')
  })

  it('scrolls the unlocking mode row into view when onNavigateToMode is not supplied', async () => {
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView

    const game = {
      id: 'game-1',
      modes: [
        {
          id: 'mode-1',
          modeKey: 'standard',
          displayName: 'Standard',
          status: 'active',
          queues: [],
          seats: [],
          queuePaths: [],
          eligibility: {
            accessible: false,
            reason: 'You have no Standard-legal decks.',
            unlockModeKey: 'deck-builder',
            requirement: { __typename: 'RequirementLeaf', label: 'Standard-legal decks', current: 0, target: 1 },
          },
        },
        {
          id: 'mode-2',
          modeKey: 'deck-builder',
          displayName: 'Deck Builder',
          status: 'active',
          queues: [],
          seats: [],
          queuePaths: [],
          eligibility: { accessible: true },
        },
      ],
    }

    render(<GameModesPanel game={game} />)

    await userEvent.click(screen.getByRole('button', { name: 'You have no Standard-legal decks.' }))

    const targetRow = document.getElementById('game-mode-row-game-1-deck-builder')
    expect(targetRow).not.toBeNull()
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' })
    // Confirm it was invoked on the target row specifically (this === targetRow),
    // not just called somewhere with matching arguments.
    expect(scrollIntoView.mock.contexts[0]).toBe(targetRow)
  })

  it('scopes the default navigation target by game so two games sharing a mode key do not collide', async () => {
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView

    // Two different games on a lobby listing, each rendering its own
    // GameModesPanel, that both happen to have a mode with the same
    // modeKey ("deck-builder"). Without scoping the DOM id by game, both
    // rows would share the id "game-mode-row-deck-builder" and
    // document.getElementById would only ever resolve the first one.
    function makeGame(gameId) {
      return {
        id: gameId,
        modes: [
          {
            id: `${gameId}-mode-standard`,
            modeKey: 'standard',
            displayName: 'Standard',
            status: 'active',
            queues: [],
            seats: [],
            queuePaths: [],
            eligibility: {
              accessible: false,
              reason: 'You have no Standard-legal decks.',
              unlockModeKey: 'deck-builder',
              requirement: { __typename: 'RequirementLeaf', label: 'Standard-legal decks', current: 0, target: 1 },
            },
          },
          {
            id: `${gameId}-mode-deck-builder`,
            modeKey: 'deck-builder',
            displayName: 'Deck Builder',
            status: 'active',
            queues: [],
            seats: [],
            queuePaths: [],
            eligibility: { accessible: true },
          },
        ],
      }
    }

    const gameA = makeGame('game-a')
    const gameB = makeGame('game-b')

    render(
      <>
        <GameModesPanel game={gameA} />
        <GameModesPanel game={gameB} />
      </>,
    )

    const rowAStandard = screen.getAllByRole('button', { name: 'You have no Standard-legal decks.' })[0]
    const rowBStandard = screen.getAllByRole('button', { name: 'You have no Standard-legal decks.' })[1]

    // Click game B's locked "Standard" reason button.
    await userEvent.click(rowBStandard)

    const gameATarget = document.getElementById('game-mode-row-game-a-deck-builder')
    const gameBTarget = document.getElementById('game-mode-row-game-b-deck-builder')
    expect(gameATarget).not.toBeNull()
    expect(gameBTarget).not.toBeNull()
    expect(gameATarget).not.toBe(gameBTarget)

    // Only game B's Deck Builder row should have been scrolled into view.
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    expect(scrollIntoView.mock.contexts[0]).toBe(gameBTarget)
    expect(scrollIntoView.mock.contexts[0]).not.toBe(gameATarget)

    // Sanity: clicking game A's own locked row scrolls to game A's row.
    scrollIntoView.mockClear()
    await userEvent.click(rowAStandard)
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    expect(scrollIntoView.mock.contexts[0]).toBe(gameATarget)
  })
})

describe('GameModesPanel prominent mode-card badges', () => {
  it('shows a player-count badge when minPlayers/maxPlayers are known', () => {
    render(
      <GameModesPanel
        game={baseGame({ minPlayers: 2, maxPlayers: 4, eligibility: { accessible: true } })}
        variant="prominent"
      />,
    )
    expect(screen.getByText('2-4 players')).toBeInTheDocument()
  })

  it('omits the badge when player counts are unknown', () => {
    render(
      <GameModesPanel
        game={baseGame({ eligibility: { accessible: true } })}
        variant="prominent"
      />,
    )
    expect(screen.queryByLabelText('Mode details')).not.toBeInTheDocument()
  })
})

describe('GameModesPanel friends action', () => {
  function friendsGame() {
    return {
      id: 'game-1',
      slug: 'legendary-quest',
      modes: [
        {
          id: 'mode-1',
          modeKey: 'legendary',
          displayName: 'Legendary',
          status: 'active',
          queues: [],
          seats: [],
          queuePaths: [],
          eligibility: { accessible: true },
        },
      ],
    }
  }

  it('offers the friends action in the prototype wording', () => {
    render(<GameModesPanel game={friendsGame()} />)

    expect(screen.getByRole('button', { name: 'Play with friends' })).toBeInTheDocument()
  })

  /** Same rule as the Jump in icon: decorative, and the label stays the label. */
  it('leads Play with friends with a decorative icon that does not rename the button', () => {
    render(<GameModesPanel game={friendsGame()} />)

    const button = screen.getByRole('button', { name: 'Play with friends' })
    const icon = button.querySelector('svg')
    expect(icon).not.toBeNull()
    expect(icon).toHaveAttribute('aria-hidden', 'true')
    expect(button).toHaveAccessibleName('Play with friends')
  })

  it('takes the player to their group instead of opening the room panel', async () => {
    const user = userEvent.setup()
    const { createPrivateTable } = await import('../../lib/tables')
    createPrivateTable.mockResolvedValue({ id: 'table-1' })
    navigateTo.mockClear()

    render(<GameModesPanel game={friendsGame()} />)
    await user.click(screen.getByRole('button', { name: 'Play with friends' }))

    expect(createPrivateTable).toHaveBeenCalledWith('game-1', 'mode-1')
    expect(navigateTo).toHaveBeenCalledWith('/group')
  })
})

describe('GameModesPanel friends action for a visitor with no identity', () => {
  function friendsGame() {
    return {
      id: 'game-1',
      slug: 'legendary-quest',
      modes: [
        {
          id: 'mode-1',
          modeKey: 'legendary',
          displayName: 'Legendary',
          status: 'active',
          queues: [],
          seats: [],
          queuePaths: [],
          eligibility: { accessible: true },
        },
      ],
    }
  }

  it('refreshes the room against the session the visitor just made, not the one they clicked with', async () => {
    const user = userEvent.setup()
    const { createPrivateTable } = await import('../../lib/tables')
    navigateTo.mockClear()
    createPrivateTable.mockReset()
    createPrivateTable.mockResolvedValue({ id: 'table-1' })
    refreshBeforeIdentity.mockClear()
    refreshAfterIdentity.mockClear()
    authState.user = null

    const { rerender } = render(
      <IdentityPromptProvider>
        <GameModesPanel game={friendsGame()} />
      </IdentityPromptProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'Play with friends' }))

    authState.user = identifiedUser()
    rerender(
      <IdentityPromptProvider>
        <GameModesPanel game={friendsGame()} />
      </IdentityPromptProvider>,
    )

    await waitFor(() => expect(navigateTo).toHaveBeenCalledWith('/group'))
    // Replaying the closure captured at click time would refresh as a visitor
    // with no session, which clears the room the create just made and leaves the
    // group screen reporting itself ended.
    expect(refreshAfterIdentity).toHaveBeenCalled()
    expect(refreshBeforeIdentity).not.toHaveBeenCalled()
  })

  it('continues into the group once the visitor picks an identity', async () => {
    const user = userEvent.setup()
    const { createPrivateTable } = await import('../../lib/tables')
    navigateTo.mockClear()
    createPrivateTable.mockReset()
    createPrivateTable.mockResolvedValue({ id: 'table-1' })
    authState.user = null

    const { rerender } = render(
      <IdentityPromptProvider>
        <GameModesPanel game={friendsGame()} />
      </IdentityPromptProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'Play with friends' }))

    // Held, not lost, and no error shouted at someone who did nothing wrong. The
    // table is not created yet either — the intent waits on the prompt rather
    // than firing a mutation the backend is bound to refuse.
    expect(navigateTo).not.toHaveBeenCalled()
    expect(createPrivateTable).not.toHaveBeenCalled()
    expect(screen.queryByText(/identity required/i)).not.toBeInTheDocument()
    expect(screen.getByTestId('identity-gate')).toBeInTheDocument()

    authState.user = identifiedUser()
    rerender(
      <IdentityPromptProvider>
        <GameModesPanel game={friendsGame()} />
      </IdentityPromptProvider>,
    )

    await waitFor(() => expect(navigateTo).toHaveBeenCalledWith('/group'))
    expect(createPrivateTable).toHaveBeenCalledTimes(1)
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
  })
})

describe('GameModesPanel identity at join time', () => {
  const queuedGame = {
    id: 'game-1',
    slug: 'legendary-quest',
    name: 'Legendary Quest',
    modes: [
      {
        id: 'mode-1',
        modeKey: 'legendary',
        displayName: 'Legendary',
        status: 'active',
        queues: [{ id: 'queue-1', status: 'active' }],
        seats: [],
        queuePaths: [],
        eligibility: { accessible: true },
      },
    ],
  }

  function renderPanel() {
    return render(
      <IdentityPromptProvider>
        <GameModesPanel game={queuedGame} />
      </IdentityPromptProvider>,
    )
  }

  beforeEach(() => {
    vi.clearAllMocks()
    authState.user = null
    authState.loading = false
    authState.refreshSession = async () => {}
    joinQueue.mockResolvedValue({ queued: true, queuedCount: 1 })
  })

  it('prompts instead of joining when the visitor has no name yet', async () => {
    renderPanel()

    await userEvent.click(screen.getByRole('button', { name: 'Jump in' }))

    expect(screen.getByTestId('identity-gate')).toBeInTheDocument()
    expect(joinQueue).not.toHaveBeenCalled()
  })

  it('runs the join the visitor asked for once they have picked a name', async () => {
    const { rerender } = renderPanel()

    await userEvent.click(screen.getByRole('button', { name: 'Jump in' }))
    expect(joinQueue).not.toHaveBeenCalled()

    authState.user = identifiedUser()
    rerender(
      <IdentityPromptProvider>
        <GameModesPanel game={queuedGame} />
      </IdentityPromptProvider>,
    )

    // Third argument is the pre-queue selection; this mode declares no option
    // groups, so nothing is sent.
    await waitFor(() => expect(joinQueue).toHaveBeenCalledWith('queue-1', undefined, undefined))
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
    expect(await screen.findByText('Finding players…')).toBeInTheDocument()
  })

  it('joins straight away for a player who already has a name', async () => {
    authState.user = identifiedUser()
    renderPanel()

    await userEvent.click(screen.getByRole('button', { name: 'Jump in' }))

    // Third argument is the pre-queue selection; this mode declares no option
    // groups, so nothing is sent.
    await waitFor(() => expect(joinQueue).toHaveBeenCalledWith('queue-1', undefined, undefined))
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
  })
})
