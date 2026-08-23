import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect } from 'vitest'
import ComponentLibrarySection from './ComponentLibrarySection'

describe('ComponentLibrarySection', () => {
  it('renders the heading and defaults to the Primitives tab', () => {
    render(<ComponentLibrarySection />)
    expect(screen.getByRole('heading', { name: 'Component library' })).toBeInTheDocument()
    // "Button" appears twice by design (pill preview swatch + card title),
    // so assert presence rather than a single unique match.
    expect(screen.getAllByText('Button').length).toBeGreaterThan(0)
    expect(screen.queryByText('Genre-accent catalog card')).not.toBeInTheDocument()
  })

  it('switches to the Composite patterns tab on click', async () => {
    render(<ComponentLibrarySection />)

    await userEvent.click(screen.getByRole('button', { name: 'Composite patterns' }))

    expect(screen.getByText('Genre-accent catalog card')).toBeInTheDocument()
    expect(screen.queryByText('Button')).not.toBeInTheDocument()
  })

  it('shows status labels for both a ported and a not-started item', () => {
    render(<ComponentLibrarySection />)
    expect(screen.getAllByText('Ported').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Not started').length).toBeGreaterThan(0)
  })
})
