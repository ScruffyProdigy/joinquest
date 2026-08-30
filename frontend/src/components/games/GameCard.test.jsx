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

    // Only the left-hand genre/player badges should render.
    expect(container.querySelectorAll('.rounded-full')).toHaveLength(2)
  })

  it('omits the blurb with no shortDescription', () => {
    renderCard({ ...baseGame, shortDescription: '' })

    expect(screen.queryByText('Fast rounds, random teams, one winner.')).not.toBeInTheDocument()
  })

  it('uses the accentColor override for the card accent when set', () => {
    const { container } = renderCard({ ...baseGame, accentColor: '#7c3aed' })

    // The hero div is the img's parent; the outer shell also carries a gradient,
    // so select by structure rather than by [style*="linear-gradient"].
    // jsdom rewrites plain hex inside a gradient to rgb(), but leaves the
    // unparseable color-mix() on the shell verbatim -- hence the two forms below.
    const hero = container.querySelector('img').parentElement
    expect(hero.getAttribute('style')).toContain('rgb(124, 58, 237)')
    // 0x7c/0x3a/0xed each * 0.65 -> 0x51/0x26/0x9a, the derived dark stop
    expect(hero.getAttribute('style')).toContain('rgb(81, 38, 154)')

    const shell = container.querySelector('.rounded-2xl')
    expect(shell.getAttribute('style')).toContain('#51269a')
  })

  it('falls back to the hashed accent when no accentColor is set', () => {
    const { container } = renderCard(baseGame)
    const hero = container.querySelector('img').parentElement

    expect(hero.getAttribute('style')).toContain('rgb(249, 115, 22)')
    expect(hero.getAttribute('style')).not.toContain('rgb(124, 58, 237)')
  })
})
