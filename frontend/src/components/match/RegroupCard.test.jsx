import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import RegroupCard from './RegroupCard'

// Group play, and nobody left early: the one combination that gets the roster at all.
const base = {
  game: { name: 'Word Hunt', modes: [{ id: '1' }, { id: '2' }] },
  complete: true,
  groupPlay: true,
  participants: [
    { user: { id: 'a', displayName: 'Ada' }, regroup: 'IN', reason: 'COMPLETED' },
    { user: { id: 'b', displayName: 'Bo' }, regroup: 'PENDING', reason: 'COMPLETED' },
  ],
}

/**
 * The roster every real player actually arrives on. `playAgain` is the only thing in the
 * system that sets anyone to IN, so a roster that starts all-PENDING is the normal case, not
 * an edge one.
 */
const allPending = {
  ...base,
  participants: base.participants.map((p) => ({ ...p, regroup: 'PENDING' })),
}

describe('RegroupCard', () => {
  it('names who has not answered', () => {
    render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByText('Who’s playing again?')).toBeInTheDocument()
    expect(screen.getByText('Not back yet')).toBeInTheDocument()
  })

  it('names the viewer as "You"', () => {
    render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByText('You')).toBeInTheDocument()
    expect(screen.queryByText('Ada')).not.toBeInTheDocument()
  })

  it('reports the count', () => {
    render(<RegroupCard result={allPending} viewerId="a" minPlayers={2} />)
    expect(screen.getByTestId('regroup-count')).toHaveTextContent('0 of 2 back and in · needs 2 to start')
  })

  it('counts only IN', () => {
    const outAndIn = {
      ...base,
      participants: [
        { user: { id: 'a', displayName: 'Ada' }, regroup: 'IN', reason: 'COMPLETED' },
        { user: { id: 'b', displayName: 'Bo' }, regroup: 'OUT', reason: 'COMPLETED' },
      ],
    }
    render(<RegroupCard result={outAndIn} viewerId="b" minPlayers={2} />)
    expect(screen.getByText('Out')).toBeInTheDocument()
    expect(screen.getByTestId('regroup-count')).toHaveTextContent('1 of 2 back and in')
  })

  /**
   * The card is the roster and nothing else now — the ways out of the match live in the
   * screen's action bar, so that they can still be offered to the players this card is not
   * for (JQ-277).
   */
  it('carries no actions of its own', () => {
    render(<RegroupCard result={allPending} viewerId="a" minPlayers={2} />)
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })

  /**
   * The matrix: solo/group × played-out/exited-early. Exactly one cell renders the card, and
   * the tests below pin all four so neither axis is later collapsed into the other.
   *
   * Solo is not an oversight — a solo player has nobody to regroup with. They queued alone,
   * and the other names here are strangers they never agreed to come back with. A group came
   * in through a room, by QR code in the same place or by share link from different ones, and
   * going back to that room together is the whole expectation.
   */
  describe('who sees the roster', () => {
    const withViewerReason = (result, reason) => ({
      ...result,
      participants: result.participants.map((p) => (p.user.id === 'a' ? { ...p, reason } : p)),
    })
    const group = { ...allPending, groupPlay: true }
    const solo = { ...allPending, groupPlay: false }

    const renderCard = (result) =>
      render(<RegroupCard result={result} viewerId="a" minPlayers={2} />).container

    it('shows it to a group who played the match out', () => {
      renderCard(withViewerReason(group, 'COMPLETED'))
      expect(screen.getByText('Who’s playing again?')).toBeInTheDocument()
    })

    it.each([
      ['a group player who was eliminated', withViewerReason(group, 'ELIMINATED')],
      ['a group player who forfeited', withViewerReason(group, 'FORFEIT')],
      ['a solo player who played the match out', withViewerReason(solo, 'COMPLETED')],
      ['a solo player who was eliminated', withViewerReason(solo, 'ELIMINATED')],
    ])('renders nothing at all for %s', (_label, result) => {
      expect(renderCard(result)).toBeEmptyDOMElement()
    })

    /**
     * The card sits in one flow with the standings now rather than inside a finished-match
     * branch, so the thing that used to keep it away from a running match — where there is no
     * regroup to answer and `playAgain` would be refused — has to be its own condition
     * (JQ-277).
     */
    it('stays away until the match is actually over', () => {
      expect(renderCard({ ...group, complete: false })).toBeEmptyDOMElement()
    })

    it('keys on the viewer’s own exit, not on somebody else’s', () => {
      const othersLeftEarly = {
        ...group,
        participants: group.participants.map((p) => ({
          ...p,
          reason: p.user.id === 'a' ? 'COMPLETED' : 'ELIMINATED',
        })),
      }
      renderCard(othersLeftEarly)
      expect(screen.getByText('Who’s playing again?')).toBeInTheDocument()
    })

    // A drop is usually an accident. Losing the roster as well as the match would be a
    // second punishment for it. A null reason means the game never reported this player,
    // which is not evidence of anything.
    it.each([['a disconnect', 'DISCONNECT'], ['no reported reason', null]])(
      'keeps the roster for %s',
      (_label, reason) => {
        renderCard(withViewerReason(group, reason))
        expect(screen.getByText('Who’s playing again?')).toBeInTheDocument()
      },
    )
  })
})
