import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import AppTitle from './AppTitle'

describe('AppTitle', () => {
  it('renders a single-color JoinQuest heading with no split spans', () => {
    render(<AppTitle />)
    const heading = screen.getByRole('heading', { level: 1, name: 'JoinQuest' })
    expect(heading).toBeInTheDocument()
    expect(heading.querySelector('span')).not.toBeInTheDocument()
  })

  it('merges an extra className onto the heading', () => {
    render(<AppTitle className="extra-class" />)
    const heading = screen.getByRole('heading', { level: 1, name: 'JoinQuest' })
    expect(heading.className).toContain('app-title')
    expect(heading.className).toContain('extra-class')
  })
})
