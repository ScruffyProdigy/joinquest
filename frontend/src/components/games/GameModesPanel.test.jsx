import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import GameModesPanel from './GameModesPanel'

vi.mock('../rooms/ActiveRoomProvider', () => ({
  useActiveRoom: () => ({
    refresh: vi.fn(),
    openRoom: vi.fn(),
  }),
}))

vi.mock('../../lib/tables', () => ({
  createPrivateTable: vi.fn(),
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

    const targetRow = document.getElementById('game-mode-row-deck-builder')
    expect(targetRow).not.toBeNull()
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' })
    // Confirm it was invoked on the target row specifically (this === targetRow),
    // not just called somewhere with matching arguments.
    expect(scrollIntoView.mock.contexts[0]).toBe(targetRow)
  })
})
