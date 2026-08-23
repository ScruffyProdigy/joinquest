import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import DeveloperCatalogMetadata from './DeveloperCatalogMetadata'
import { updateMyGameMetadata } from '../../lib/developers'

vi.mock('../../lib/developers', () => ({
  fetchCatalogTagTaxonomy: vi.fn().mockResolvedValue([]),
  updateMyGameMetadata: vi.fn().mockResolvedValue({}),
}))

const baseGame = {
  id: 'game-1',
  name: 'Accent Game',
  shortDescription: 'Short',
  longDescription: 'Long',
  howToPlay: 'How',
  contactEmail: 'dev@example.com',
  websiteUrl: '',
  communityUrl: '',
  tags: [],
  accentColor: '#7c3aed',
}

describe('DeveloperCatalogMetadata accent color', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the saved accent color and submits it unchanged', async () => {
    const user = userEvent.setup()
    render(<DeveloperCatalogMetadata game={baseGame} />)

    expect(screen.getByLabelText('Accent color hex value')).toHaveValue('#7c3aed')

    await user.click(screen.getByRole('button', { name: 'Save listing' }))

    expect(updateMyGameMetadata).toHaveBeenCalledWith(
      expect.objectContaining({ accentColor: '#7c3aed' }),
    )
  })

  it('normalizes #rgb shorthand for the swatch instead of falling back to the default color', async () => {
    const user = userEvent.setup()
    render(<DeveloperCatalogMetadata game={baseGame} />)

    const hexField = screen.getByLabelText('Accent color hex value')
    await user.clear(hexField)
    await user.type(hexField, '#abc')

    expect(screen.getByLabelText('Catalog accent color (optional)')).toHaveValue('#aabbcc')
  })

  it('sends an empty accentColor after "Use default" so the override is cleared', async () => {
    const user = userEvent.setup()
    render(<DeveloperCatalogMetadata game={baseGame} />)

    await user.click(screen.getByRole('button', { name: 'Use default' }))
    expect(screen.getByLabelText('Accent color hex value')).toHaveValue('')

    await user.click(screen.getByRole('button', { name: 'Save listing' }))

    expect(updateMyGameMetadata).toHaveBeenCalledWith(
      expect.objectContaining({ accentColor: '' }),
    )
  })
})
