import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import GameCard from './GameCard'

const baseGame = {
  id: 'game-1',
  slug: 'trivia-blitz',
  name: 'Trivia Blitz',
  heroUrl: '/games/trivia-blitz-hero.jpg',
  shortDescription: 'Fast rounds, random teams, one winner.',
  genre: 'words-trivia',
  modes: [{ id: 'mode-1', status: 'active', minPlayers: 2, maxPlayers: 8 }],
}

function renderCard(game) {
  return render(
    <ul>
      <GameCard game={game} />
    </ul>,
  )
}

/** The card art, ignoring the blurred copy that bleeds out from under the card edge. */
function heroImage(container) {
  return container.querySelector('a img, [class*="inset-px"] img')
}

describe('GameCard', () => {
  it('renders hero art, title, metadata pills, and blurb', () => {
    const { container } = renderCard(baseGame)

    expect(heroImage(container)).toHaveAttribute('src', '/games/trivia-blitz-hero.jpg?v=1')
    expect(screen.getByRole('heading', { name: 'Trivia Blitz' })).toBeInTheDocument()
    expect(screen.getByText('Words & Trivia')).toBeInTheDocument()
    expect(screen.getByText('2–8')).toBeInTheDocument()
    expect(screen.getByText('Fast rounds, random teams, one winner.')).toBeInTheDocument()
  })

  it('orders the metadata row genre, players, duration', () => {
    renderCard({ ...baseGame, durationMinutes: 12 })

    const labels = screen.getByText('Words & Trivia').closest('div')
    expect(labels.textContent).toBe('Words & Trivia2–812 min')
  })

  it('drops the duration slot entirely while no game carries one', () => {
    renderCard(baseGame)

    expect(screen.queryByText(/min$/)).not.toBeInTheDocument()
    // Genre and players only -- no third, empty pill holding the slot open.
    expect(screen.getByText('Words & Trivia').closest('div').children).toHaveLength(2)
  })

  it('uses catalogHeroUrl over heroUrl when set', () => {
    const { container } = renderCard({
      ...baseGame,
      catalogHeroUrl: '/games/trivia-blitz-catalog.webp',
    })

    expect(heroImage(container)).toHaveAttribute('src', '/games/trivia-blitz-catalog.webp?v=1')
  })

  it('falls back to the placeholder hero rather than breaking the card', () => {
    const { container } = renderCard({ ...baseGame, heroUrl: '' })

    expect(heroImage(container)).toHaveAttribute('src', '/games/default-hero.svg?v=1')
    expect(screen.getByRole('heading', { name: 'Trivia Blitz' })).toBeInTheDocument()
  })

  it('composites the wordmark over the art at its placement', () => {
    const { container } = renderCard({
      ...baseGame,
      titleArt: { url: '/games/trivia-blitz-title.webp', anchor: 'bottom-left', widthPct: 70 },
    })

    const mark = container.querySelector('img[src*="-title.webp"]')
    expect(mark).toHaveAttribute('src', '/games/trivia-blitz-title.webp?v=1')
    expect(mark.getAttribute('style')).toContain('width: 70%')
    expect(mark.getAttribute('style')).toContain('bottom: 6.8%')
    // The heading still carries the name -- a mark the scrim covers must not be the
    // only place the game is named.
    expect(screen.getByRole('heading', { name: 'Trivia Blitz' })).toBeInTheDocument()
  })

  it('draws no wordmark for a game without one', () => {
    const { container } = renderCard(baseGame)

    expect(container.querySelector('img[src*="-title."]')).toBeNull()
  })

  it('ignores a wordmark whose placement is unusable', () => {
    const { container } = renderCard({
      ...baseGame,
      titleArt: { url: '/games/trivia-blitz-title.webp', anchor: 'sideways', widthPct: 70 },
    })

    expect(container.querySelector('img[src*="-title.webp"]')).toBeNull()
  })

  it('links the whole card to the game detail page', () => {
    renderCard(baseGame)

    expect(screen.getByRole('link')).toHaveAttribute('href', '/games/trivia-blitz')
  })

  it('renders as a non-interactive card with no slug', () => {
    renderCard({ ...baseGame, slug: undefined })

    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Trivia Blitz' })).toBeInTheDocument()
  })

  it('omits the genre pill when the game declares no genre', () => {
    renderCard({ ...baseGame, genre: null })

    expect(screen.queryByText('Words & Trivia')).not.toBeInTheDocument()
  })

  it('omits the player pill with no active modes', () => {
    renderCard({ ...baseGame, modes: [] })

    expect(screen.queryByText('2–8')).not.toBeInTheDocument()
  })

  it('renders the live-activity pill', () => {
    renderCard({ ...baseGame, playerActivity: { playing: 4, queued: 2 } })

    expect(screen.getByText('4 playing')).toBeInTheDocument()
  })

  it('shows the queue instead when nobody is in a session yet', () => {
    renderCard({ ...baseGame, playerActivity: { playing: 0, queued: 3 } })

    expect(screen.getByText('3 waiting')).toBeInTheDocument()
    expect(screen.queryByText('0 playing')).not.toBeInTheDocument()
  })

  it('omits the live pill for a quiet game', () => {
    const { container } = renderCard(baseGame)

    // Only the genre and player pills should render.
    expect(container.querySelectorAll('.rounded-full')).toHaveLength(2)
  })

  it('omits the blurb with no shortDescription', () => {
    renderCard({ ...baseGame, shortDescription: '' })

    expect(screen.queryByText('Fast rounds, random teams, one winner.')).not.toBeInTheDocument()
  })

  it('uses the accentColor override for the card fill when set', () => {
    const { container } = renderCard({ ...baseGame, accentColor: '#7c3aed' })

    // jsdom rewrites plain hex inside a gradient to rgb().
    const shell = container.querySelector('[class*="inset-px"]')
    expect(shell.getAttribute('style')).toContain('rgb(124, 58, 237)')
    // 0x7c/0x3a/0xed each * 0.65 -> 0x51/0x26/0x9a, the derived dark stop
    expect(shell.getAttribute('style')).toContain('rgb(81, 38, 154)')
  })

  it('falls back to the hashed accent when no accentColor is set', () => {
    const { container } = renderCard(baseGame)
    const shell = container.querySelector('[class*="inset-px"]')

    expect(shell.getAttribute('style')).toContain('rgb(249, 115, 22)')
    expect(shell.getAttribute('style')).not.toContain('rgb(124, 58, 237)')
  })
})
