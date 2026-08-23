import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import GameDetailPage from './GameDetailPage'
import * as games from '../../lib/games'

vi.mock('../../lib/games', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchGameBySlug: vi.fn(),
  }
})

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => ({ user: { id: 'user-1' }, loading: false }),
}))

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
  fetchMyQueueStatus: vi.fn().mockResolvedValue({ queued: false }),
  leaveQueue: vi.fn(),
  prefetchSubscriptionAuth: vi.fn().mockResolvedValue('Bearer test'),
  subscribeToQueue: vi.fn().mockResolvedValue(() => {}),
}))

const detailGame = {
  id: 'game-wh',
  slug: 'word-hunt',
  name: 'Word Hunt',
  iconUrl: '/games/word-hunt-icon.png',
  heroUrl: '/games/word-hunt-hero.jpg',
  shortDescription: 'Party word game on a shared grid.',
  longDescription: 'Word Hunt is a competitive party word game on a shared grid.',
  howToPlay: 'Give one-word clues. Guess tiles to score from the pot.',
  tutorialUrl: 'https://word-hunt-arena.win',
  screenshots: ['/games/word-hunt-shot-1.png'],
  tags: ['party'],
  modes: [
    {
      id: 'mode-wh',
      modeKey: 'party',
      displayName: 'Party',
      status: 'active',
      seats: [{ queuePath: null }, { queuePath: null }],
      queues: [{ id: 'queue-wh', name: 'Default', status: 'active' }],
    },
  ],
}

describe('GameDetailPage', () => {
  beforeEach(() => {
    vi.mocked(games.fetchGameBySlug).mockReset()
  })

  it('renders game detail content', async () => {
    vi.mocked(games.fetchGameBySlug).mockResolvedValue(detailGame)

    render(
      <GameDetailPage
        slug="word-hunt"
        activeIntent={null}
        activeTableSeat={null}
        onQueueChange={vi.fn()}
        onTableChange={vi.fn()}
      />,
    )

    expect(await screen.findByText('Word Hunt')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '← Back' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument()
    expect(screen.getByText(detailGame.longDescription)).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'How to play' })).toBeInTheDocument()
    expect(screen.getByText(detailGame.howToPlay)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open tutorial' })).toHaveAttribute(
      'href',
      'https://word-hunt-arena.win',
    )
    expect(screen.getByTestId('game-detail-hero')).toHaveAttribute(
      'src',
      '/games/word-hunt-hero.jpg?v=1',
    )
    expect(screen.getByRole('button', { name: 'Look for group' })).toBeInTheDocument()
  })

  it('shows not found when the slug is unknown', async () => {
    vi.mocked(games.fetchGameBySlug).mockResolvedValue(null)

    render(
      <GameDetailPage
        slug="missing-game"
        activeIntent={null}
        activeTableSeat={null}
        onQueueChange={vi.fn()}
        onTableChange={vi.fn()}
      />,
    )

    expect(await screen.findByRole('heading', { name: 'Game not found' })).toBeInTheDocument()
  })

  it('falls back to shortDescription when longDescription is missing', async () => {
    vi.mocked(games.fetchGameBySlug).mockResolvedValue({
      ...detailGame,
      longDescription: '',
    })

    render(
      <GameDetailPage
        slug="word-hunt"
        activeIntent={null}
        activeTableSeat={null}
        onQueueChange={vi.fn()}
        onTableChange={vi.fn()}
      />,
    )

    await waitFor(() => {
      expect(screen.getByText('Party word game on a shared grid.')).toBeInTheDocument()
    })
  })
})
