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

describe('RegroupCard', () => {
  it('names who has not answered', () => {
    render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByText('Who’s playing again?')).toBeInTheDocument()
    expect(screen.getByText('Not back yet')).toBeInTheDocument()
  })

  it('disables the primary action until enough players are in', () => {
    render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByRole('button', { name: 'Need 1 more to play again' })).toBeDisabled()
  })

  it('enables it once the count is met', () => {
    const ready = { ...base, participants: base.participants.map((p) => ({ ...p, regroup: 'IN' })) }
    render(<RegroupCard result={ready} viewerId="a" minPlayers={2} />)
    expect(screen.getByRole('button', { name: 'Another round' })).toBeEnabled()
  })

  it('offers the mode switch only for multi-mode games', () => {
    const { unmount } = render(<RegroupCard result={base} viewerId="a" minPlayers={2} />)
    expect(screen.getByRole('button', { name: 'Back to Word Hunt' })).toBeInTheDocument()
    unmount()

    const singleMode = { ...base, game: { ...base.game, modes: [{ id: '1' }] } }
    render(<RegroupCard result={singleMode} viewerId="a" minPlayers={2} />)
    expect(screen.queryByRole('button', { name: 'Back to Word Hunt' })).not.toBeInTheDocument()
  })

  it('counts only IN — a player who opted out never unlocks the round', () => {
    const outAndIn = {
      ...base,
      participants: [
        { user: { id: 'a', displayName: 'Ada' }, regroup: 'IN' },
        { user: { id: 'b', displayName: 'Bo' }, regroup: 'OUT' },
      ],
    }
    render(<RegroupCard result={outAndIn} viewerId="a" minPlayers={2} />)
    expect(screen.getByText('Out')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Need 1 more to play again' })).toBeDisabled()
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
    const ready = { ...base, participants: base.participants.map((p) => ({ ...p, regroup: 'IN' })) }
    render(
      <RegroupCard
        result={ready}
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
    const ready = { ...base, participants: base.participants.map((p) => ({ ...p, regroup: 'IN' })) }
    render(<RegroupCard result={ready} viewerId="a" minPlayers={2} busy />)
    expect(screen.getByRole('button', { name: 'Another round' })).toBeDisabled()
  })
})
