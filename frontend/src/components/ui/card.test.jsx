import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { Card, CardHeader, CardTitle, CardContent, CardFooter } from './card'

describe('Card', () => {
  it('renders its children', () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>Title</CardTitle>
        </CardHeader>
        <CardContent>Body</CardContent>
      </Card>,
    )
    expect(screen.getByText('Title')).toBeInTheDocument()
    expect(screen.getByText('Body')).toBeInTheDocument()
  })

  it('applies the card background token class', () => {
    const { container } = render(<Card>content</Card>)
    expect(container.querySelector('[data-slot="card"]')).toHaveClass('bg-card')
  })

  it('applies an explicit border-border class so the border color is never legacy-CSS-dependent', () => {
    const { container } = render(<Card>content</Card>)
    expect(container.querySelector('[data-slot="card"]')).toHaveClass('border-border')
  })

  it('merges a custom className', () => {
    render(<Card className="max-w-lg">content</Card>)
    expect(screen.getByText('content')).toHaveClass('max-w-lg', 'bg-card')
  })

  it('CardTitle renders as a plain div by default', () => {
    render(<CardTitle>Account settings</CardTitle>)
    const title = screen.getByText('Account settings')
    expect(title.tagName).toBe('DIV')
  })

  it('CardTitle renders as a heading via the as prop', () => {
    render(<CardTitle as="h1">Account settings</CardTitle>)
    expect(screen.getByRole('heading', { level: 1, name: 'Account settings' })).toBeInTheDocument()
  })

  it('composes header, content, and footer slots', () => {
    render(
      <Card>
        <CardHeader data-testid="header" />
        <CardContent data-testid="content" />
        <CardFooter data-testid="footer" />
      </Card>,
    )
    expect(screen.getByTestId('header')).toHaveAttribute('data-slot', 'card-header')
    expect(screen.getByTestId('content')).toHaveAttribute('data-slot', 'card-content')
    expect(screen.getByTestId('footer')).toHaveAttribute('data-slot', 'card-footer')
  })
})
