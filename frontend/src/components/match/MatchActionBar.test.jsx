import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import MatchActionBar, { RegroupActions } from './MatchActionBar'

const base = {
  game: { id: 'g1', slug: 'word-hunt', name: 'Word Hunt', modes: [{ id: '1' }, { id: '2' }] },
  groupPlay: true,
  participants: [
    { user: { id: 'a', displayName: 'Ada' }, regroup: 'PENDING', reason: 'COMPLETED' },
    { user: { id: 'b', displayName: 'Bo' }, regroup: 'PENDING', reason: 'COMPLETED' },
  ],
}

const renderActions = (result = base, props = {}) =>
  render(
    <MatchActionBar>
      <RegroupActions result={result} viewerId="a" {...props} />
    </MatchActionBar>,
  )

describe('RegroupActions', () => {
  /**
   * The regression the whole flow rests on. `playAgain` is the only writer of IN and this
   * button is its only caller, so a disabled primary is a permanent deadlock: two players
   * both landing here PENDING would each wait forever for the other (JQ-183).
   */
  it('offers an enabled way in when nobody has opted in yet', () => {
    renderActions()
    expect(screen.getByRole('button', { name: 'Another round' })).toBeEnabled()
  })

  it('sends a player who is already in back to the table instead of asking again', () => {
    const viewerIn = {
      ...base,
      participants: base.participants.map((p) => (p.user.id === 'a' ? { ...p, regroup: 'IN' } : p)),
    }
    renderActions(viewerIn)
    expect(screen.getByRole('button', { name: 'Back to the table' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: 'Another round' })).not.toBeInTheDocument()
  })

  it('always offers a way out', () => {
    renderActions()
    expect(screen.getByRole('button', { name: 'Find something new' })).toBeInTheDocument()
  })

  /**
   * The `modes.length > 1` gate is gone (JQ-233). It used to leave a single-mode game — an
   * RPSLR duel — with no way back into the game at all: wait for someone who had already
   * left, or go to the catalog. That is the case that needs the way back most.
   */
  it('offers the way back into the game however many modes it has', () => {
    const { unmount } = renderActions()
    expect(screen.getByRole('button', { name: 'Back to Word Hunt' })).toBeInTheDocument()
    unmount()

    renderActions({ ...base, game: { ...base.game, modes: [{ id: '1' }] } })
    expect(screen.getByRole('button', { name: 'Back to Word Hunt' })).toBeInTheDocument()
  })

  it('paints the primary with the game’s own accent gradient', () => {
    renderActions()
    expect(screen.getByRole('button', { name: 'Another round' })).toHaveStyle({
      background: 'linear-gradient(135deg, #f59e0b, #b45309)',
    })
  })

  it('disables every action while a regroup request is in flight', () => {
    renderActions(base, { busy: true })
    expect(screen.getByRole('button', { name: 'Another round' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Find something new' })).toBeDisabled()
  })

  it('wires each action to its own handler', async () => {
    const user = userEvent.setup()
    const onPlayAgain = vi.fn()
    const onDecline = vi.fn()
    const onBackToGame = vi.fn()
    renderActions(base, { onPlayAgain, onDecline, onBackToGame })

    await user.click(screen.getByRole('button', { name: 'Another round' }))
    await user.click(screen.getByRole('button', { name: 'Back to Word Hunt' }))
    await user.click(screen.getByRole('button', { name: 'Find something new' }))

    expect(onPlayAgain).toHaveBeenCalledTimes(1)
    expect(onBackToGame).toHaveBeenCalledTimes(1)
    expect(onDecline).toHaveBeenCalledTimes(1)
  })

  /**
   * The solo rule and the group rule are different by design (JQ-232). A solo player's
   * primary action replays the role and options they just had, so they are the only ones who
   * need a way to reach those choices again — a group was never given them back.
   */
  describe('the solo second action', () => {
    const solo = { ...base, groupPlay: false, mode: { hasPreMatchChoice: true } }

    it('offers "Choose again" to a solo player whose mode has something to choose', () => {
      renderActions(solo)
      expect(screen.getByRole('button', { name: 'Choose again' })).toBeEnabled()
    })

    it('leaves it out entirely when the mode has nothing to choose', () => {
      renderActions({ ...solo, mode: { hasPreMatchChoice: false } })
      expect(screen.queryByRole('button', { name: 'Choose again' })).not.toBeInTheDocument()
    })

    it('never offers it to a group, whose choices were not carried forward anyway', () => {
      renderActions({ ...solo, groupPlay: true })
      expect(screen.queryByRole('button', { name: 'Choose again' })).not.toBeInTheDocument()
    })

    // Both lead back to the game, so offering both would be two buttons and one
    // destination. The more specific promise wins.
    it('replaces "Back to <game>" rather than sitting beside it', () => {
      renderActions(solo)
      expect(screen.queryByRole('button', { name: 'Back to Word Hunt' })).not.toBeInTheDocument()
    })

    it('wires to its own handler', async () => {
      const user = userEvent.setup()
      const onChooseAgain = vi.fn()
      const onBackToGame = vi.fn()
      renderActions(solo, { onChooseAgain, onBackToGame })

      await user.click(screen.getByRole('button', { name: 'Choose again' }))

      expect(onChooseAgain).toHaveBeenCalledTimes(1)
      expect(onBackToGame).not.toHaveBeenCalled()
    })
  })

  /**
   * A solo player and a player who exited early get no roster card at all, so without the bar
   * being part of the screen rather than part of that card they would have no way off this
   * screen (JQ-277).
   */
  it('offers the full set to a solo player, who has no roster card to hold them', () => {
    renderActions({ ...base, groupPlay: false })
    expect(screen.getByRole('button', { name: 'Another round' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Back to Word Hunt' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Find something new' })).toBeInTheDocument()
  })
})
