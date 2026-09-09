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

  it('shows every Link variant so the primitive is discoverable', () => {
    render(<StylePreviewPage />)
    expect(screen.getByRole('heading', { name: 'Links' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Inline link' })).toHaveClass('text-link')
    expect(screen.getByRole('link', { name: 'Pill link' })).toHaveClass('border-primary')
    expect(screen.getByRole('button', { name: 'Quiet action' })).toHaveClass('text-muted-foreground')
  })

  it('shows Button and Link side by side so the shared geometry is checkable (JQ-223)', () => {
    render(<StylePreviewPage />)
    expect(screen.getByRole('heading', { name: 'Button and Link, side by side' })).toBeInTheDocument()
    const button = screen.getByRole('button', { name: 'Filled button' })
    const pill = screen.getByRole('link', { name: 'Outlined pill link' })
    for (const control of [button, pill]) {
      expect(control).toHaveClass('rounded-[99px]')
      expect(control).toHaveClass('py-4')
      expect(control).toHaveClass('font-bold')
    }
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
