import { render, screen } from '@testing-library/react'
import { act } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import GroupSpectatorList from './GroupSpectatorList'

const you = { user: { id: 'u1', displayName: 'Ada' }, status: 'here' }
const member = { user: { id: 'u2', displayName: 'Bo' }, status: 'here' }
const awaiting = { user: { id: 'u3', displayName: 'Cy' }, status: 'awaiting' }

afterEach(() => {
  vi.useRealTimers()
})

describe('GroupSpectatorList', () => {
  it('stays out of the way when nobody but the viewer is here', () => {
    const { container } = render(<GroupSpectatorList players={[you]} userId="u1" />)
    expect(container).toBeEmptyDOMElement()
  })

  it('stays out of the way when there is nobody to list at all', () => {
    const { container } = render(<GroupSpectatorList players={[]} userId="u1" />)
    expect(container).toBeEmptyDOMElement()
  })

  it('appears once someone else is still picking', () => {
    render(<GroupSpectatorList players={[you, member]} userId="u1" />)
    expect(screen.getByText('Picking a seat')).toBeInTheDocument()
    expect(screen.getByText('You')).toBeInTheDocument()
    expect(screen.getByText('Bo')).toBeInTheDocument()
  })

  it('appears when the only other person is a previous player who has not answered', () => {
    render(<GroupSpectatorList players={[you, awaiting]} userId="u1" />)
    expect(screen.getByText('Picking a seat')).toBeInTheDocument()
  })

  it('marks a previous player awaiting, and does not mark anyone else', () => {
    render(<GroupSpectatorList players={[member, awaiting]} userId="u1" />)
    const rows = screen.getAllByRole('listitem')
    expect(rows).toHaveLength(2)
    expect(rows[0]).not.toHaveTextContent('Awaiting')
    expect(rows[1]).toHaveTextContent('Awaiting')
  })

  it('says Out when it watches someone decline, then drops the row', () => {
    vi.useFakeTimers()
    const { rerender } = render(<GroupSpectatorList players={[member, awaiting]} userId="u1" />)

    rerender(<GroupSpectatorList players={[member, { ...awaiting, status: 'out' }]} userId="u1" />)
    expect(screen.getByText('Cy')).toBeInTheDocument()
    expect(screen.getByText('Out')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(3000)
    })
    expect(screen.queryByText('Cy')).not.toBeInTheDocument()
  })

  /**
   * A decline that happened before the page loaded is history, not news. Flashing "Out"
   * at someone who just arrived would announce a decision they never saw being made.
   */
  it('never flashes Out for someone who had already declined on first render', () => {
    render(<GroupSpectatorList players={[member, { ...awaiting, status: 'out' }]} userId="u1" />)
    expect(screen.queryByText('Cy')).not.toBeInTheDocument()
    expect(screen.queryByText('Out')).not.toBeInTheDocument()
  })
})
