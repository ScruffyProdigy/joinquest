import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import StillPlaying from './StillPlaying'

describe('StillPlaying', () => {
  it('lists the others as in progress', () => {
    const participants = [
      { user: { id: 'b', displayName: 'Bo' }, role: 'Guesser' },
      { user: { id: 'c', displayName: 'Cy' }, role: null },
    ]
    render(<StillPlaying participants={participants} />)
    expect(screen.getByText('Bo')).toBeInTheDocument()
    expect(screen.getByText('Cy')).toBeInTheDocument()
    expect(screen.getAllByText('In progress…')).toHaveLength(2)
  })

  it('renders cleanly with no participants', () => {
    render(<StillPlaying participants={[]} />)
    expect(screen.queryByText('In progress…')).not.toBeInTheDocument()
  })
})
