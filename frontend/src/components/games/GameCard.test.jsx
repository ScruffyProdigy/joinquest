import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import GameCard from './GameCard'

const baseGame = {
  id: 'game-1',
  slug: 'trivia-blitz',
  name: 'Trivia Blitz',
  heroUrl: '/games/trivia-blitz-hero.jpg',
  shortDescription: 'Fast rounds, random teams, one winner.',
  tags: ['Trivia', 'Party'],
  modes: [{ id: 'mode-1', status: 'active', minPlayers: 2, maxPlayers: 8 }],
}

function renderCard(game) {
  return render(
    <ul>
      <GameCard game={game} />
    </ul>,
  )
}

describe('GameCard', () => {
  it('renders hero art, title, badges, and blurb', () => {
    const { container } = renderCard(baseGame)

    expect(container.querySelector('img')).toHaveAttribute(
      'src',
      '/games/trivia-blitz-hero.jpg?v=1',
    )
    expect(screen.getByRole('heading', { name: 'Trivia Blitz' })).toBeInTheDocument()
    expect(screen.getByText('Trivia · Party')).toBeInTheDocument()
    expect(screen.getByText('2–8 players')).toBeInTheDocument()
    expect(screen.getByText('Fast rounds, random teams, one winner.')).toBeInTheDocument()
  })

  it('uses catalogHeroUrl over heroUrl when set', () => {
    const { container } = renderCard({ ...baseGame, catalogHeroUrl: '/games/trivia-blitz-catalog.png' })

    expect(container.querySelector('img')).toHaveAttribute(
      'src',
      '/games/trivia-blitz-catalog.png?v=1',
    )
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

  it('omits the genre/mode badge with no tags', () => {
    renderCard({ ...baseGame, tags: [] })

    expect(screen.queryByText('Trivia · Party')).not.toBeInTheDocument()
  })

  it('omits the player-count badge with no active modes', () => {
    renderCard({ ...baseGame, modes: [] })

    expect(screen.queryByText('2–8 players')).not.toBeInTheDocument()
  })

  it('omits the blurb with no shortDescription', () => {
    renderCard({ ...baseGame, shortDescription: '' })

    expect(screen.queryByText('Fast rounds, random teams, one winner.')).not.toBeInTheDocument()
  })
})
