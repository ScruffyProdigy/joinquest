import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import RegroupCard from './RegroupCard'

// Group play, and nobody left early: the one combination that still gets the roster.
const base = {
  game: { name: 'Word Hunt', modes: [{ id: '1' }, { id: '2' }] },
  groupPlay: true,
  participants: [
    { user: { id: 'a', displayName: 'Ada' }, regroup: 'IN', reason: 'COMPLETED' },
    { user: { id: 'b', displayName: 'Bo' }, regroup: 'PENDING', reason: 'COMPLETED' },
  ],
}

/**
 * The roster every real player actually arrives on. `playAgain` is the only thing in the
 * system that sets anyone to IN, and this card's primary button is its only caller, so a
 * roster that starts all-PENDING is the one state the card must never refuse to act on.
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

  it('offers an enabled way in when nobody has opted in yet', () => {
    render(<RegroupCard result={allPending} viewerId="a" minPlayers={2} />)
    expect(screen.getByRole('button', { name: 'Another round' })).toBeEnabled()
  })

  it('reports the count without gating anything on it', () => {
    render(<RegroupCard result={allPending} viewerId="a" minPlayers={2} />)
    expect(screen.getByTestId('regroup-count')).toHaveTextContent('0 of 2 back and in · needs 2 to start')
    expect(screen.getByRole('button', { name: 'Another round' })).toBeEnabled()
  })

  it('sends a player who is already in back to the table instead of asking again', () => {
    render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByRole('button', { name: 'Back to the table' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: 'Another round' })).not.toBeInTheDocument()
  })

  /**
   * The `modes.length > 1` gate is gone (JQ-233). It used to leave a single-mode game — an
   * RPSLR duel — with no way back into the game at all: wait for someone who had already
   * left, or go to the catalog. That is the case that needs the way back most.
   */
  it('offers the way back into the game however many modes it has', () => {
    const { unmount } = render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByRole('button', { name: 'Back to Word Hunt' })).toBeInTheDocument()
    unmount()

    const singleMode = { ...base, game: { ...base.game, modes: [{ id: '1' }] } }
    render(<RegroupCard result={singleMode} viewerId="a" minPlayers={2} />)
    expect(screen.getByRole('button', { name: 'Back to Word Hunt' })).toBeInTheDocument()
  })

  it('counts only IN, and still lets an opted-out player change their mind', () => {
    const outAndIn = {
      ...base,
      participants: [
        { user: { id: 'a', displayName: 'Ada' }, regroup: 'IN' },
        { user: { id: 'b', displayName: 'Bo' }, regroup: 'OUT' },
      ],
    }
    render(<RegroupCard result={outAndIn} viewerId="b" minPlayers={2} />)
    expect(screen.getByText('Out')).toBeInTheDocument()
    expect(screen.getByTestId('regroup-count')).toHaveTextContent('1 of 2 back and in')
    expect(screen.getByRole('button', { name: 'Another round' })).toBeEnabled()
  })

  it('names the viewer as "You" and always offers a way out', () => {
    render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByText('You')).toBeInTheDocument()
    expect(screen.queryByText('Ada')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Find something new' })).toBeInTheDocument()
  })

  it('wires each action to its own handler', async () => {
    const user = userEvent.setup()
    const onPlayAgain = vi.fn()
    const onDecline = vi.fn()
    const onBackToGame = vi.fn()
    render(
      <RegroupCard
        result={allPending}
        viewerId="a"
        minPlayers={2}
        onPlayAgain={onPlayAgain}
        onDecline={onDecline}
        onBackToGame={onBackToGame}
      />,
    )

    await user.click(screen.getByRole('button', { name: 'Another round' }))
    await user.click(screen.getByRole('button', { name: 'Back to Word Hunt' }))
    await user.click(screen.getByRole('button', { name: 'Find something new' }))

    expect(onPlayAgain).toHaveBeenCalledTimes(1)
    expect(onDecline).toHaveBeenCalledTimes(1)
    expect(onBackToGame).toHaveBeenCalledTimes(1)
  })

  it('disables the primary action while a regroup request is in flight', () => {
    render(<RegroupCard result={allPending} viewerId="a" minPlayers={2} busy />)
    expect(screen.getByRole('button', { name: 'Another round' })).toBeDisabled()
  })

  /**
   * The solo rule and the group rule are different by design (JQ-232). A solo player's
   * primary action replays the role and options they just had, so they are the only ones who
   * need a way to reach those choices again — a group was never given them back.
   */
  describe('the solo second action', () => {
    const solo = { ...allPending, groupPlay: false, mode: { hasPreMatchChoice: true } }

    it('offers "Choose again" to a solo player whose mode has something to choose', () => {
      render(<RegroupCard result={solo} viewerId="a" minPlayers={2} />)
      expect(screen.getByRole('button', { name: 'Choose again' })).toBeEnabled()
    })

    it('leaves it out entirely when the mode has nothing to choose', () => {
      const nothingToChoose = { ...solo, mode: { hasPreMatchChoice: false } }
      render(<RegroupCard result={nothingToChoose} viewerId="a" minPlayers={2} />)
      expect(screen.queryByRole('button', { name: 'Choose again' })).not.toBeInTheDocument()
    })

    it('never offers it to a group, whose choices were not carried forward anyway', () => {
      const group = { ...solo, groupPlay: true }
      render(<RegroupCard result={group} viewerId="a" minPlayers={2} />)
      expect(screen.queryByRole('button', { name: 'Choose again' })).not.toBeInTheDocument()
    })

    // Both lead back to the game, so offering both would be two buttons and one
    // destination. The more specific promise wins.
    it('replaces "Back to <game>" rather than sitting beside it', () => {
      render(<RegroupCard result={solo} viewerId="a" minPlayers={2} />)
      expect(screen.queryByRole('button', { name: 'Back to Word Hunt' })).not.toBeInTheDocument()
    })

    it('wires to its own handler', async () => {
      const user = userEvent.setup()
      const onChooseAgain = vi.fn()
      const onBackToGame = vi.fn()
      render(
        <RegroupCard
          result={solo}
          viewerId="a"
          minPlayers={2}
          onChooseAgain={onChooseAgain}
          onBackToGame={onBackToGame}
        />,
      )

      await user.click(screen.getByRole('button', { name: 'Choose again' }))

      expect(onChooseAgain).toHaveBeenCalledTimes(1)
      expect(onBackToGame).not.toHaveBeenCalled()
    })
  })
  /**
   * The matrix JQ-233 asks for: solo/group × played-out/exited-early. Exactly one cell shows
   * the roster, and the tests below pin all four so neither axis is later collapsed into the
   * other.
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

    const expectRoster = () => {
      expect(screen.getByText('Who’s playing again?')).toBeInTheDocument()
      expect(screen.getByTestId('regroup-count')).toBeInTheDocument()
      expect(screen.getByText('You')).toBeInTheDocument()
    }
    const expectNoRoster = () => {
      expect(screen.getByText('Play again?')).toBeInTheDocument()
      expect(screen.queryByTestId('regroup-count')).not.toBeInTheDocument()
      expect(screen.queryByText('You')).not.toBeInTheDocument()
      expect(screen.queryByText('Bo')).not.toBeInTheDocument()
    }

    it('shows it to a group who played the match out', () => {
      render(<RegroupCard result={withViewerReason(group, 'COMPLETED')} viewerId="a" minPlayers={2} />)
      expectRoster()
    })

    it('withholds it from a group player who was eliminated', () => {
      render(<RegroupCard result={withViewerReason(group, 'ELIMINATED')} viewerId="a" minPlayers={2} />)
      expectNoRoster()
    })

    it('withholds it from a solo player who played the match out', () => {
      render(<RegroupCard result={withViewerReason(solo, 'COMPLETED')} viewerId="a" minPlayers={2} />)
      expectNoRoster()
    })

    it('withholds it from a solo player who was eliminated', () => {
      render(<RegroupCard result={withViewerReason(solo, 'ELIMINATED')} viewerId="a" minPlayers={2} />)
      expectNoRoster()
    })

    it('keys on the viewer’s own exit, not on somebody else’s', () => {
      const othersLeftEarly = {
        ...group,
        participants: group.participants.map((p) => ({
          ...p,
          reason: p.user.id === 'a' ? 'COMPLETED' : 'ELIMINATED',
        })),
      }
      render(<RegroupCard result={othersLeftEarly} viewerId="a" minPlayers={2} />)
      expectRoster()
    })

    it('treats a forfeit as an early exit too', () => {
      render(<RegroupCard result={withViewerReason(group, 'FORFEIT')} viewerId="a" minPlayers={2} />)
      expectNoRoster()
    })

    // A drop is usually an accident. Losing the roster as well as the match would be a
    // second punishment for it.
    it('keeps the roster for a disconnect', () => {
      render(<RegroupCard result={withViewerReason(group, 'DISCONNECT')} viewerId="a" minPlayers={2} />)
      expectRoster()
    })

    // Null means the game never reported this player, which is not evidence of anything.
    it('keeps the roster when the game reported no reason at all', () => {
      render(<RegroupCard result={withViewerReason(group, null)} viewerId="a" minPlayers={2} />)
      expectRoster()
    })

    /**
     * The one thing that must survive every cell. `playAgain` is the only writer of IN, and
     * this button is its only caller, so a disabled primary is a deadlock wherever it appears
     * — quorum or no quorum, roster or no roster (JQ-183, *Do not copy the prototype here*).
     */
    it.each([
      ['group, played out', withViewerReason(group, 'COMPLETED')],
      ['group, exited early', withViewerReason(group, 'ELIMINATED')],
      ['solo, played out', withViewerReason(solo, 'COMPLETED')],
      ['solo, exited early', withViewerReason(solo, 'ELIMINATED')],
    ])('leaves the primary action enabled and the exits intact — %s', (_label, result) => {
      render(<RegroupCard result={result} viewerId="a" minPlayers={2} />)
      expect(screen.getByRole('button', { name: 'Another round' })).toBeEnabled()
      expect(screen.getByRole('button', { name: 'Find something new' })).toBeInTheDocument()
    })
  })
})
