import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import MatchStandings from './MatchStandings'

const result = {
  complete: true,
  reported: true,
  participants: [
    { user: { id: 'a', displayName: 'Ada' }, placement: 1, winner: true, finished: true, regroup: 'IN' },
    { user: { id: 'b', displayName: 'Bo' }, placement: null, winner: false, finished: false, regroup: 'PENDING' },
  ],
}

describe('MatchStandings', () => {
  it('marks the winner and the unfinished player', () => {
    render(<MatchStandings result={result} viewerId="a" />)
    expect(screen.getByText('Final standings')).toBeInTheDocument()
    expect(screen.getByText('Winner')).toBeInTheDocument()
    expect(screen.getByText('Still playing')).toBeInTheDocument()
  })

  it('titles the card differently while the match runs', () => {
    render(<MatchStandings result={{ ...result, complete: false }} viewerId="a" />)
    expect(screen.getByText('Results so far')).toBeInTheDocument()
  })

  it('degrades to the roster when the game reported nothing', () => {
    const bare = {
      complete: true,
      reported: false,
      participants: [{ user: { id: 'a', displayName: 'Ada' }, placement: null, winner: false, finished: true, regroup: 'PENDING' }],
    }
    // viewerId is a non-participant here (not 'a') so this exercises the
    // degraded-roster path in isolation from the "You" substitution, which
    // has its own test below.
    render(<MatchStandings result={bare} viewerId="someone-else" />)
    expect(screen.getByText('Ada')).toBeInTheDocument()
    expect(screen.queryByText('Winner')).not.toBeInTheDocument()
  })

  it('shows "You" and tints the viewer\'s own row', () => {
    render(<MatchStandings result={result} viewerId="a" />)
    expect(screen.getByText('You')).toBeInTheDocument()
    expect(screen.queryByText('Ada')).not.toBeInTheDocument()
  })

  it('renders a viewer row cleanly when the participant has no role', () => {
    const noRole = {
      complete: true,
      reported: true,
      participants: [{ user: { id: 'a', displayName: 'Ada' }, placement: 1, winner: true, finished: true, regroup: 'IN' }],
    }
    render(<MatchStandings result={noRole} viewerId="a" />)
    expect(screen.getByText('You')).toBeInTheDocument()
  })

  it('sorts known placements first, then the rest in their original order', () => {
    const mixed = {
      complete: false,
      reported: true,
      participants: [
        { user: { id: 'c', displayName: 'Cy' }, placement: null, winner: false, finished: false, regroup: 'PENDING' },
        { user: { id: 'b', displayName: 'Bo' }, placement: 2, winner: false, finished: true, regroup: 'IN' },
        { user: { id: 'd', displayName: 'Di' }, placement: null, winner: false, finished: false, regroup: 'PENDING' },
        { user: { id: 'a', displayName: 'Ada' }, placement: 1, winner: true, finished: true, regroup: 'IN' },
      ],
    }
    render(<MatchStandings result={mixed} viewerId="z" />)
    const names = screen.getAllByText(/^(Ada|Bo|Cy|Di)$/).map((el) => el.textContent)
    expect(names).toEqual(['Ada', 'Bo', 'Cy', 'Di'])
  })
})
