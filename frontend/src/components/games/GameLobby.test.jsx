import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import GameLobby from './GameLobby'
import { AuthProvider } from '../auth/AuthProvider'
import { ActiveRoomProvider } from '../rooms/ActiveRoomProvider'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../../test/setup'
import * as games from '../../lib/games'
import { DEVELOPER_PROMO_TITLE, DEVELOPER_PROMO_TITLE_NO_RESULTS } from '../../lib/playerCopy'

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
        <GameLobby headingId="find-a-game-heading" />
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

  describe('search', () => {
    function mockTwoGames() {
      vi.mocked(games.fetchGames).mockResolvedValue([
        {
          id: 'game-1',
          slug: 'spyfall',
          name: 'Spyfall',
          iconUrl: '/games/spyfall-icon.png',
          heroUrl: '/games/spyfall-hero.jpg',
          tags: ['Social', 'Deduction'],
          createdAt: '2026-01-01T00:00:00Z',
          modes: [],
        },
        {
          id: 'game-2',
          slug: 'word-ladder',
          name: 'Word Ladder',
          iconUrl: '/games/word-ladder-icon.png',
          heroUrl: '/games/word-ladder-hero.jpg',
          tags: ['Word', 'Strategy'],
          createdAt: '2026-01-02T00:00:00Z',
          modes: [],
        },
      ])
    }

    it('shows a labelled search box above the grid', async () => {
      mockAuthenticatedSession()
      renderGameLobby()

      const input = await screen.findByLabelText('Search games')
      expect(input).toHaveAttribute('placeholder', 'Search games\u2026')
    })

    it('does not show the search box when the catalog is empty', async () => {
      mockAuthenticatedSession()
      vi.mocked(games.fetchGames).mockResolvedValue([])
      renderGameLobby()

      expect(await screen.findByText('No games available yet.')).toBeInTheDocument()
      expect(screen.queryByLabelText('Search games')).not.toBeInTheDocument()
    })

    it('filters the grid as the player types, and restores it when cleared', async () => {
      mockAuthenticatedSession()
      mockTwoGames()
      renderGameLobby()

      const input = await screen.findByLabelText('Search games')
      await userEvent.type(input, 'spy')

      expect(screen.getByRole('heading', { name: 'Spyfall' })).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'Word Ladder' })).not.toBeInTheDocument()

      await userEvent.clear(input)

      expect(screen.getByRole('heading', { name: 'Spyfall' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Word Ladder' })).toBeInTheDocument()
    })

    it('matches on tags as well as titles', async () => {
      mockAuthenticatedSession()
      mockTwoGames()
      renderGameLobby()

      const input = await screen.findByLabelText('Search games')
      await userEvent.type(input, 'deduction')

      expect(screen.getByRole('heading', { name: 'Spyfall' })).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'Word Ladder' })).not.toBeInTheDocument()
    })

    it('shows an empty state when nothing matches', async () => {
      mockAuthenticatedSession()
      mockTwoGames()
      renderGameLobby()

      const input = await screen.findByLabelText('Search games')
      await userEvent.type(input, 'zzzzz')

      expect(screen.getByText('No games found')).toBeInTheDocument()
      expect(screen.getByText('Try a different search term')).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'Spyfall' })).not.toBeInTheDocument()
    })

    it('is available to signed-out visitors too', async () => {
      mockUnauthenticatedSession()
      mockTwoGames()
      renderGameLobby()

      const input = await screen.findByLabelText('Search games')
      await userEvent.type(input, 'word')

      expect(screen.getByRole('heading', { name: 'Word Ladder' })).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: 'Spyfall' })).not.toBeInTheDocument()
    })
  })
  // JQ-71: the promo used to be a stacked block below the catalog. It now sits in the
  // list itself, and JQ-41 settled that the dev portal stays open to everyone.
  describe('developer promo card', () => {
    function mockGames(count) {
      vi.mocked(games.fetchGames).mockResolvedValue(
        Array.from({ length: count }, (_, i) => ({
          id: `game-${i + 1}`,
          slug: `game-${i + 1}`,
          name: `Game ${i + 1}`,
          iconUrl: `/games/game-${i + 1}-icon.png`,
          heroUrl: `/games/game-${i + 1}-hero.jpg`,
          tags: [],
          createdAt: '2026-01-01T00:00:00Z',
          modes: [],
        })),
      )
    }

    function promoLink() {
      return screen.queryByRole('link', { name: /get started for developers/i })
    }

    it('shows the promo to a signed-out visitor', async () => {
      mockUnauthenticatedSession()
      renderGameLobby()

      await waitFor(() => expect(promoLink()).toHaveAttribute('href', '/developers'))
    })

    it('shows the promo to a guest', async () => {
      mockAuthenticatedSession({
        id: 'guest-1',
        email: null,
        displayName: null,
        isGuest: true,
        createdAt: '2026-01-01T00:00:00Z',
      })
      renderGameLobby()

      await waitFor(() => expect(promoLink()).toHaveAttribute('href', '/developers'))
    })

    it('shows the promo to a signed-in account', async () => {
      mockAuthenticatedSession()
      renderGameLobby()

      await waitFor(() => expect(promoLink()).toHaveAttribute('href', '/developers'))
    })

    it('sits in a fixed slot regardless of how many games load', async () => {
      mockAuthenticatedSession()
      mockGames(10)
      renderGameLobby()

      await waitFor(() => expect(promoLink()).toBeInTheDocument())
      // Fourth item: three game cards ahead of it, the rest behind.
      const items = screen.getAllByRole('listitem')
      expect(items[3]).toContainElement(promoLink())
      expect(items).toHaveLength(11)
    })

    it('falls to the end of a catalog shorter than its slot', async () => {
      mockAuthenticatedSession()
      mockGames(2)
      renderGameLobby()

      await waitFor(() => expect(promoLink()).toBeInTheDocument())
      const items = screen.getAllByRole('listitem')
      expect(items).toHaveLength(3)
      expect(items[2]).toContainElement(promoLink())
    })

    it('rides along through a filtered search', async () => {
      mockAuthenticatedSession()
      mockGames(10)
      renderGameLobby()

      const input = await screen.findByLabelText('Search games')
      await userEvent.type(input, 'Game 1')

      expect(promoLink()).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: DEVELOPER_PROMO_TITLE })).toBeInTheDocument()
    })

    // Matches the prototype: the dead end becomes the pitch.
    it('retitles itself when the search clears the list, and is all that is left', async () => {
      mockAuthenticatedSession()
      mockGames(10)
      renderGameLobby()

      const input = await screen.findByLabelText('Search games')
      await userEvent.type(input, 'zzzzz')

      expect(screen.getByText('No games found')).toBeInTheDocument()
      expect(
        screen.getByRole('heading', { name: DEVELOPER_PROMO_TITLE_NO_RESULTS }),
      ).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: DEVELOPER_PROMO_TITLE })).not.toBeInTheDocument()
      expect(screen.getAllByRole('listitem')).toHaveLength(1)
    })

    it('goes back to its usual title once the search clears', async () => {
      mockAuthenticatedSession()
      mockGames(10)
      renderGameLobby()

      const input = await screen.findByLabelText('Search games')
      await userEvent.type(input, 'zzzzz')
      await userEvent.clear(input)

      expect(screen.getByRole('heading', { name: DEVELOPER_PROMO_TITLE })).toBeInTheDocument()
      expect(
        screen.queryByRole('heading', { name: DEVELOPER_PROMO_TITLE_NO_RESULTS }),
      ).not.toBeInTheDocument()
    })

    it('is absent when the catalog itself is empty', async () => {
      mockAuthenticatedSession()
      vi.mocked(games.fetchGames).mockResolvedValue([])
      renderGameLobby()

      expect(await screen.findByText('No games available yet.')).toBeInTheDocument()
      expect(promoLink()).not.toBeInTheDocument()
    })
  })
})
