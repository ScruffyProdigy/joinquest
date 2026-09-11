import { render, screen } from '@testing-library/react'
import { act } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import GroupSpectatorList from './GroupSpectatorList'

const you = { user: { id: 'u1', displayName: 'Ada' }, status: 'here' }
const member = { user: { id: 'u2', displayName: 'Bo' }, status: 'here' }
const awaiting = { user: { id: 'u3', displayName: 'Cy' }, status: 'awaiting' }
const awayMember = { user: { id: 'u4', displayName: 'Dee' }, status: 'here', away: true }
const awayAwaiting = { user: { id: 'u5', displayName: 'Eli' }, status: 'awaiting', away: true }

afterEach(() => {
  vi.useRealTimers()
})

describe('GroupSpectatorList', () => {
  /**
   * The prototype seeds this list with `You: in-lobby` and only removes the row when the
   * seat is claimed, so a player who has just arrived sees themselves waiting. Production
   * used to hide a viewer-only card on the grounds that it said nothing; it said the one
   * thing that mattered — you have not sat down yet.
   */
  it('shows the viewer their own row while they hold no seat', () => {
    render(<GroupSpectatorList players={[you]} userId="u1" />)
    expect(screen.getByText('Picking a seat')).toBeInTheDocument()
    expect(screen.getByText('You')).toBeInTheDocument()
  })

  // JQ-265. The caption is the assertion and the dimming only supports it: a faded avatar
  // next to a full-strength name is a rendering artefact to anyone not comparing rows, and
  // it says nothing at all to a screen reader. The name stays legible either way — the
  // doubt is about whether they are watching, not about who they are.
  it('captions an away member and keeps their name legible', () => {
    const { container } = render(<GroupSpectatorList players={[member, awayMember]} userId="u1" />)
    expect(screen.getByText('Dee')).toBeInTheDocument()
    expect(screen.getByText('Away')).toBeInTheDocument()
    const dimmed = container.querySelectorAll('[data-slot="avatar"][data-away="true"]')
    expect(dimmed).toHaveLength(1)
    expect(dimmed[0].getAttribute('title')).toBe('Dee (away)')
  })

  it('says nothing about presence when everyone is here', () => {
    render(<GroupSpectatorList players={[you, member]} userId="u1" />)
    expect(screen.queryByText('Away')).toBeNull()
  })

  // Away and awaiting are different questions — whether we believe they are there, and
  // what they have said about the next match — so a player whose phone died mid-decision
  // has to read as both rather than have one reading swallow the other. This is the case
  // that makes the caption necessary rather than nice: two states in one list, and only
  // one of them carried words.
  it('shows a player who is both awaiting and away as both', () => {
    const { container } = render(<GroupSpectatorList players={[member, awayAwaiting]} userId="u1" />)
    expect(screen.getByText('Awaiting')).toBeInTheDocument()
    expect(screen.getByText('Away')).toBeInTheDocument()
    expect(container.querySelector('[data-slot="avatar"][data-away="true"]')).toBeTruthy()
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

  it('lists the rows bare, with no bullet indent pushing them off the card edge', () => {
    // Tailwind's theme and utilities load without preflight (see tailwind.css), so a
    // `ul` keeps the browser's disc marker and 40px indent unless it opts out.
    render(<GroupSpectatorList players={[you, member]} userId="u1" />)

    const list = screen.getByRole('list')
    expect(list).toHaveClass('list-none')
    expect(list).toHaveClass('p-0')
    expect(list).toHaveClass('m-0')
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
