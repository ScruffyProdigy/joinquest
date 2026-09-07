import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import YourResult from './YourResult'

const result = {
  complete: false,
  participants: [
    { user: { id: 'a', displayName: 'Ada' }, reason: 'COMPLETED', placement: 2 },
    { user: { id: 'b', displayName: 'Bo' }, reason: null, placement: null },
    { user: { id: 'c', displayName: 'Cy' }, reason: null, placement: null },
    { user: { id: 'd', displayName: 'Di' }, reason: null, placement: null },
  ],
}

describe('YourResult', () => {
  it('gives the viewer their own headline while the match runs on', () => {
    render(<YourResult result={result} viewerId="a" />)
    expect(screen.getByText('Your result so far')).toBeInTheDocument()
    expect(screen.getByText('2nd place.')).toBeInTheDocument()
    expect(screen.getByText('Waiting for others to finish.')).toBeInTheDocument()
  })

  it('calls out an elimination rather than the generic wait', () => {
    const eliminated = {
      ...result,
      participants: result.participants.map((participant) =>
        participant.user.id === 'a' ? { ...participant, reason: 'ELIMINATED', placement: 4 } : participant,
      ),
    }
    render(<YourResult result={eliminated} viewerId="a" />)
    expect(screen.getByText('4th of 4.')).toBeInTheDocument()
    expect(screen.getByText('Eliminated before the end.')).toBeInTheDocument()
  })

  // ordinal(null) is "nullth", so an unplaced viewer is the case that would ship visible
  // nonsense rather than a missing line.
  it('never renders an ordinal for a viewer the game has not placed', () => {
    render(<YourResult result={result} viewerId="b" />)
    expect(screen.getByText('Placement not yet known')).toBeInTheDocument()
    expect(screen.getByText('Waiting for others to finish.')).toBeInTheDocument()
    expect(screen.queryByText(/nullth/)).not.toBeInTheDocument()
  })

  it('renders nothing when the viewer is not in the roster', () => {
    const { container } = render(<YourResult result={result} viewerId="stranger" />)
    expect(container).toBeEmptyDOMElement()
  })
})
