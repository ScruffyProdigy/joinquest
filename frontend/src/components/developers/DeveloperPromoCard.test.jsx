import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import DeveloperPromoCard from './DeveloperPromoCard'
import { DEVELOPER_PROMO_TITLE } from '../../lib/playerCopy'

function renderPromoCard() {
  return render(
    <ul>
      <DeveloperPromoCard />
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
})
