import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { OptionButton } from './option-button'

describe('OptionButton', () => {
  it('renders a non-submitting button carrying its label', () => {
    render(<OptionButton>Fox</OptionButton>)
    const button = screen.getByRole('button', { name: 'Fox' })
    expect(button.tagName).toBe('BUTTON')
    expect(button).toHaveAttribute('type', 'button')
  })

  it('lays out as a full-width pill row by default', () => {
    render(<OptionButton>Fox</OptionButton>)
    const button = screen.getByRole('button')
    expect(button).toHaveClass('w-full')
    expect(button).toHaveClass('rounded-full')
    expect(button).toHaveClass('text-left')
  })

  it('stacks into a rounded tile when asked', () => {
    render(<OptionButton variant="tile">Fox</OptionButton>)
    const button = screen.getByRole('button')
    expect(button).toHaveClass('flex-col')
    expect(button).toHaveClass('rounded-2xl')
  })

  it('reports selection to assistive tech rather than by colour alone', () => {
    render(<OptionButton selected>Fox</OptionButton>)
    const button = screen.getByRole('button')
    expect(button).toHaveAttribute('aria-pressed', 'true')
    expect(button).toHaveClass('border-primary')
  })

  it('leaves aria-pressed off entirely when selection is not a concept', () => {
    render(<OptionButton>Fox</OptionButton>)
    expect(screen.getByRole('button')).not.toHaveAttribute('aria-pressed')
  })

  it('marks an unselected option as such once selection is in play', () => {
    render(<OptionButton selected={false}>Fox</OptionButton>)
    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'false')
  })

  it('carries the shared focus ring the other primitives use', () => {
    render(<OptionButton>Fox</OptionButton>)
    expect(screen.getByRole('button')).toHaveClass('focus-visible:ring-[3px]')
  })
})
