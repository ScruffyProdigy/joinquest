import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Avatar, AvatarImage, AvatarFallback } from './avatar'

describe('Avatar', () => {
  it('renders a rounded-full container', () => {
    render(<Avatar data-testid="avatar" />)
    expect(screen.getByTestId('avatar')).toHaveClass('rounded-full')
  })

  it('applies the md size by default', () => {
    render(<Avatar data-testid="avatar" />)
    expect(screen.getByTestId('avatar')).toHaveClass('size-12')
  })

  it('applies the sm size when requested', () => {
    render(<Avatar data-testid="avatar" size="sm" />)
    expect(screen.getByTestId('avatar')).toHaveClass('size-8')
  })

  it('renders AvatarImage and AvatarFallback with their data-slots', () => {
    render(
      <Avatar>
        <AvatarImage src="/x.png" alt="" data-testid="image" />
        <AvatarFallback data-testid="fallback">R</AvatarFallback>
      </Avatar>,
    )
    expect(screen.getByTestId('image')).toHaveAttribute('data-slot', 'avatar-image')
    expect(screen.getByTestId('fallback')).toHaveAttribute('data-slot', 'avatar-fallback')
  })
})
