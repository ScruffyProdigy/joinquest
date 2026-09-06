import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import YourGamesStrip from './YourGamesStrip'
import * as developers from '../../lib/developers'

vi.mock('../../lib/developers', async (importOriginal) => {
  const actual = await importOriginal()
  return { ...actual, fetchMyGames: vi.fn() }
})

// JQ-71: this used to render embedded inside DeveloperHomeBlock. That block is gone,
// so it now stands on its own below the catalog.
describe('YourGamesStrip', () => {
  beforeEach(() => {
    vi.mocked(developers.fetchMyGames).mockReset()
  })

  it('lists the developer’s games as a labelled section', async () => {
    vi.mocked(developers.fetchMyGames).mockResolvedValue([
      { id: 'game-1', name: 'Spyfall', visibility: 'PUBLIC' },
    ])

    render(<YourGamesStrip />)

    const section = await screen.findByRole('region', { name: 'Your games' })
    expect(section).toContainElement(screen.getByRole('button', { name: /Spyfall/ }))
  })

  it('renders nothing when the player has no games', async () => {
    vi.mocked(developers.fetchMyGames).mockResolvedValue([])

    const { container } = render(<YourGamesStrip />)

    await waitFor(() => expect(developers.fetchMyGames).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when the lookup fails', async () => {
    vi.mocked(developers.fetchMyGames).mockRejectedValue(new Error('nope'))

    const { container } = render(<YourGamesStrip />)

    await waitFor(() => expect(developers.fetchMyGames).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })
})
