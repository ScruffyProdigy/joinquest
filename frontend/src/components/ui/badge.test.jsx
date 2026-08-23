import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { Badge } from './badge'

describe('Badge', () => {
  it('renders its label', () => {
    render(<Badge>party</Badge>)
    expect(screen.getByText('party')).toBeInTheDocument()
  })

  it('applies the secondary variant background class when requested', () => {
    render(<Badge variant="secondary">party</Badge>)
    expect(screen.getByText('party')).toHaveClass('bg-secondary')
  })
})
