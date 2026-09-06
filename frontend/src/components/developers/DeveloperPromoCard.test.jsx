import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import DeveloperPromoCard from './DeveloperPromoCard'
import { DEVELOPER_PROMO_TITLE, DEVELOPER_PROMO_TITLE_NO_RESULTS } from '../../lib/playerCopy'

function renderPromoCard(props = {}) {
  return render(
    <ul>
      <DeveloperPromoCard {...props} />
    </ul>,
  )
}

describe('DeveloperPromoCard', () => {
  it('links to the developer portal', () => {
    renderPromoCard()

    expect(screen.getByRole('link', { name: /get started for developers/i })).toHaveAttribute(
      'href',
      '/developers',
    )
  })

  it('names itself as a promo rather than a game', () => {
    renderPromoCard()

    expect(screen.getByRole('heading', { name: DEVELOPER_PROMO_TITLE })).toBeInTheDocument()
  })

  // Criterion: reads as a promo, not a playable game.
  it('carries no player count or duration pill', () => {
    const { container } = renderPromoCard()

    expect(container.textContent).not.toMatch(/player|min\b|\bnow playing\b/i)
  })

  it('renders as a list item so it sits in the catalog list', () => {
    renderPromoCard()

    expect(screen.getByRole('listitem')).toBeInTheDocument()
  })
  it('retitles itself when a search leaves nothing to show', () => {
    renderPromoCard({ noResults: true })

    expect(
      screen.getByRole('heading', { name: DEVELOPER_PROMO_TITLE_NO_RESULTS }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: DEVELOPER_PROMO_TITLE })).not.toBeInTheDocument()
  })

  it('keeps the same destination in both states', () => {
    renderPromoCard({ noResults: true })

    expect(screen.getByRole('link', { name: /get started for developers/i })).toHaveAttribute(
      'href',
      '/developers',
    )
  })
})
