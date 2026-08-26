import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import GameLobby from './GameLobby'
import { AuthProvider } from '../auth/AuthProvider'
import { ActiveRoomProvider } from '../rooms/ActiveRoomProvider'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../../test/setup'
import * as games from '../../lib/games'

vi.mock('../../lib/games', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchGames: vi.fn(),
    defaultQueueForGame: vi.fn((game) => game?.modes?.[0]?.queues?.[0] ?? null),
  }
})

function renderGameLobby() {
  return render(
    <AuthProvider>
      <ActiveRoomProvider>
        <GameLobby />
      </ActiveRoomProvider>
    </AuthProvider>,
  )
}

describe('GameLobby', () => {
  beforeEach(() => {
    vi.mocked(games.fetchGames).mockReset()
    vi.mocked(games.fetchGames).mockResolvedValue([
      {
        id: 'game-1',
        slug: 'rock-paper-scissors-lizard-robot',
        name: 'Rock Paper Scissors Lizard Robot',
        iconUrl: '/games/rpslr-icon.png',
        heroUrl: '/games/rpslr-hero.jpg',
        createdAt: '2026-01-01T00:00:00Z',
        modes: [
          {
            id: 'mode-1',
            modeKey: 'duel',
            displayName: 'Duel',
            status: 'active',
            seats: [{ queuePath: null }, { queuePath: null }],
            queues: [{ id: 'queue-1', name: 'Default', status: 'active' }],
          },
        ],
      },
    ])
  })

  it('shows the catalog to signed-out visitors', async () => {
    mockUnauthenticatedSession()
    renderGameLobby()

    expect(await screen.findByRole('heading', { name: 'Available games' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Rock Paper Scissors Lizard Robot' })).toBeInTheDocument()
  })

  it('queries the catalog without a player id when signed out', async () => {
    mockUnauthenticatedSession()
    renderGameLobby()

    await waitFor(() => {
      expect(games.fetchGames).toHaveBeenCalledTimes(1)
    })
    // No player id means the query drops the per-user eligibility field.
    expect(games.fetchGames).toHaveBeenCalledWith('')
  })

  it('loads and shows catalog games for signed-in users', async () => {
    mockAuthenticatedSession()
    renderGameLobby()

    expect(await screen.findByRole('heading', { name: 'Available games' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Rock Paper Scissors Lizard Robot' })).toBeInTheDocument()
    expect(games.fetchGames).toHaveBeenCalledTimes(1)
    expect(games.fetchGames).toHaveBeenCalledWith('user-1')
  })

  it('shows an error when loading games fails', async () => {
    mockAuthenticatedSession()
    vi.mocked(games.fetchGames).mockRejectedValue(new Error('API unavailable'))

    renderGameLobby()

    expect(await screen.findByText('API unavailable')).toBeInTheDocument()
  })
})
