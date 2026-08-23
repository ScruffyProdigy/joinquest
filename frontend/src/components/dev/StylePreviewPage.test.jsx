import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import StylePreviewPage from './StylePreviewPage'

describe('StylePreviewPage', () => {
  it('renders the type scale section with all 8 steps labeled', () => {
    render(<StylePreviewPage />)
    expect(screen.getByRole('heading', { name: 'Type scale' })).toBeInTheDocument()
    expect(screen.getByText(/2xs.*11px/)).toBeInTheDocument()
    expect(screen.getByText(/3xl.*40px/)).toBeInTheDocument()
  })

  it('renders the spacing section', () => {
    render(<StylePreviewPage />)
    expect(screen.getByRole('heading', { name: 'Spacing' })).toBeInTheDocument()
    expect(screen.getByText(/Tailwind's default/)).toBeInTheDocument()
  })

  it('renders all 10 per-game accent swatches', () => {
    render(<StylePreviewPage />)
    expect(screen.getByRole('heading', { name: 'Per-game accents' })).toBeInTheDocument()
    ;['amber', 'orange', 'lime', 'emerald', 'cyan', 'blue', 'indigo', 'violet', 'fuchsia', 'rose'].forEach(
      (name) => {
        expect(screen.getByText(name)).toBeInTheDocument()
      },
    )
  })
})
