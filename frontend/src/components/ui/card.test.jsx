import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { Card, CardHeader, CardTitle, CardContent } from './card'

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
})
