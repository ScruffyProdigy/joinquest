import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import RegroupCard from './RegroupCard'

const base = {
  game: { name: 'Word Hunt', modes: [{ id: '1' }, { id: '2' }] },
  participants: [
    { user: { id: 'a', displayName: 'Ada' }, regroup: 'IN' },
    { user: { id: 'b', displayName: 'Bo' }, regroup: 'PENDING' },
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

  it('offers the mode switch only for multi-mode games', () => {
    const { unmount } = render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByRole('button', { name: 'Back to Word Hunt' })).toBeInTheDocument()
    unmount()

    const singleMode = { ...base, game: { ...base.game, modes: [{ id: '1' }] } }
    render(<RegroupCard result={singleMode} viewerId="a" minPlayers={2} />)
    expect(screen.queryByRole('button', { name: 'Back to Word Hunt' })).not.toBeInTheDocument()
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
})
