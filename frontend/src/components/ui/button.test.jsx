import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Button } from './button'

describe('Button', () => {
  it('renders as a button with its label', () => {
    render(<Button>Click me</Button>)
    expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument()
  })

  it('applies the primary background class by default', () => {
    render(<Button>Primary</Button>)
    expect(screen.getByRole('button')).toHaveClass('bg-primary')
  })

  it('applies the secondary variant class when requested', () => {
    render(<Button variant="secondary">Secondary</Button>)
    expect(screen.getByRole('button')).toHaveClass('bg-secondary')
  })

  it('applies explicit background, text, and border classes for the outline variant', () => {
    render(<Button variant="outline">Outline</Button>)
    const button = screen.getByRole('button')
    expect(button).toHaveClass('bg-background')
    expect(button).toHaveClass('text-foreground')
    expect(button).toHaveClass('border-border')
  })

  it('applies explicit transparent background and text classes for the ghost variant', () => {
    render(<Button variant="ghost">Ghost</Button>)
    const button = screen.getByRole('button')
    expect(button).toHaveClass('bg-transparent')
    expect(button).toHaveClass('text-foreground')
    expect(button).toHaveClass('border-transparent')
  })

  it('has no link variant -- link-styled controls belong to the Link component', () => {
    render(<Button variant="link">Link</Button>)
    expect(screen.getByRole('button')).not.toHaveClass('underline')
  })

  it('is a pill at every size, matching the prototype (JQ-223)', () => {
    render(
      <>
        <Button>Default</Button>
        <Button size="sm">Small</Button>
        <Button size="chip">Chip</Button>
        <Button size="icon">×</Button>
      </>,
    )
    for (const button of screen.getAllByRole('button')) {
      expect(button).toHaveClass('rounded-[99px]')
    }
  })

  it('is full width by default, so a call site never passes w-full by hand', () => {
    render(<Button>Primary</Button>)
    const button = screen.getByRole('button')
    expect(button).toHaveClass('w-full')
    expect(button).toHaveClass('py-4')
  })

  it('stays compact at the sizes meant for inline rows', () => {
    render(
      <>
        <Button size="sm">Small</Button>
        <Button size="chip">Chip</Button>
      </>,
    )
    for (const button of screen.getAllByRole('button')) {
      expect(button).not.toHaveClass('w-full')
    }
  })

  it('sizes by padding rather than a fixed height, so a chip or wrapping label fits', () => {
    render(<Button size="sm">Small</Button>)
    expect(screen.getByRole('button').className).not.toMatch(/(^|\s)h-\d/)
  })

  it('presses with opacity, matching the prototype rather than a colour shift', () => {
    render(<Button>Primary</Button>)
    expect(screen.getByRole('button')).toHaveClass('active:opacity-80')
  })

  it('presses the card variant with a scale, as the prototype does for sign-in rows', () => {
    render(<Button variant="card">Continue with Google</Button>)
    const button = screen.getByRole('button')
    expect(button).toHaveClass('active:scale-[0.99]')
    expect(button).toHaveClass('bg-card')
    expect(button).not.toHaveClass('active:opacity-80')
  })
})
