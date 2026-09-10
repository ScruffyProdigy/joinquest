import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import GameQueueActions from './GameQueueActions'

describe('GameQueueActions', () => {
  it('shows a Jump in button for single-bucket modes', () => {
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="idle"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.queryByRole('region', { name: 'Jump in' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Jump in' })).toBeInTheDocument()
  })

  /**
   * The prototype leads the CTA with a lucide `zap`. It has to stay decorative:
   * an icon that joins the accessible name renames the button for anyone using
   * a screen reader or driving the app by voice.
   */
  it('leads Jump in with a decorative icon that does not rename the button', () => {
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="idle"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    const button = screen.getByRole('button', { name: 'Jump in' })
    const icon = button.querySelector('svg')
    expect(icon).not.toBeNull()
    expect(icon).toHaveAttribute('aria-hidden', 'true')
    expect(button).toHaveAccessibleName('Jump in')
  })

  it('shows waiting controls without a Jump in panel in fifo modes', () => {
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="waiting"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.queryByRole('region', { name: 'Jump in' })).not.toBeInTheDocument()
    expect(screen.getByText('Finding players…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Stop' })).toBeInTheDocument()
  })

  it('shows role buttons inside a Jump in panel for composition modes', () => {
    render(
      <GameQueueActions
        joinOptions={{
          kind: 'composition',
          paths: [
            { queuePath: 'DPS', displayName: 'DPS' },
            { queuePath: 'Support', displayName: 'Support' },
            { queuePath: 'Tank', displayName: 'Tank' },
          ],
        }}
        queueState="idle"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.getByRole('region', { name: 'Jump in' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join as DPS' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join as Support' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join as Tank' })).toBeInTheDocument()
  })

  it('uses displayName labels when provided', () => {
    render(
      <GameQueueActions
        joinOptions={{
          kind: 'composition',
          paths: [
            { queuePath: 'ClueGiver', displayName: 'Clue Giver' },
            { queuePath: 'Guesser', displayName: 'Guesser' },
          ],
        }}
        queueState="idle"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.getByRole('button', { name: 'Join as Clue Giver' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join as Guesser' })).toBeInTheDocument()
  })

  it('calls onJoin with the selected queue path', async () => {
    const onJoin = vi.fn()
    render(
      <GameQueueActions
        joinOptions={{
          kind: 'composition',
          paths: [
            { queuePath: 'DPS', displayName: 'DPS' },
            { queuePath: 'Tank', displayName: 'Tank' },
          ],
        }}
        queueState="idle"
        busy={false}
        onJoin={onJoin}
        onLeave={vi.fn()}
      />,
    )

    await userEvent.click(screen.getByRole('button', { name: 'Join as Tank' }))
    expect(onJoin).toHaveBeenCalledWith('Tank')
  })

  it('falls back to a plain Jump in button when composition paths are empty', () => {
    render(
      <GameQueueActions
        joinOptions={{
          kind: 'composition',
          paths: [{ queuePath: '', displayName: '' }],
        }}
        queueState="idle"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.queryByRole('region', { name: 'Jump in' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Jump in' })).toBeInTheDocument()
  })

  it('shows waiting state with selected role in composition panel', () => {
    render(
      <GameQueueActions
        joinOptions={{
          kind: 'composition',
          paths: [{ queuePath: 'DPS', displayName: 'Damage' }],
        }}
        queueState="waiting"
        selectedQueuePath="DPS"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.getByText('Finding players as Damage…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Stop' })).toBeInTheDocument()
  })

  it('keeps other role join buttons visible while waiting', async () => {
    const onJoin = vi.fn()
    render(
      <GameQueueActions
        joinOptions={{
          kind: 'composition',
          paths: [
            { queuePath: 'ClueGiver', displayName: 'Clue Giver' },
            { queuePath: 'Guesser', displayName: 'Guesser' },
          ],
        }}
        queueState="waiting"
        selectedQueuePath="ClueGiver"
        busy={false}
        onJoin={onJoin}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.getByText('Finding players as Clue Giver…')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Join as Clue Giver' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join as Guesser' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Join as Guesser' }))
    expect(onJoin).toHaveBeenCalledWith('Guesser')
  })

  it('shows a Single player button instead of Jump in for solo modes', () => {
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="idle"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
        solo
      />,
    )

    expect(screen.getByRole('button', { name: 'Single player' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Jump in' })).not.toBeInTheDocument()
  })

  it('shows a Starting… label while a solo mode join is in flight', () => {
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="idle"
        busy
        onJoin={vi.fn()}
        onLeave={vi.fn()}
        solo
      />,
    )

    expect(screen.getByRole('button', { name: 'Starting…' })).toBeInTheDocument()
  })

  it('calls onJoin with no queue path when Single player is clicked in a solo mode', async () => {
    const onJoin = vi.fn()
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="idle"
        busy={false}
        onJoin={onJoin}
        onLeave={vi.fn()}
        solo
      />,
    )

    await userEvent.click(screen.getByRole('button', { name: 'Single player' }))
    expect(onJoin).toHaveBeenCalledWith()
  })

  it('shows Launch game without a Leave match button once a solo mode fires', () => {
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="matched"
        joinUrl="https://example.com/launch"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
        solo
      />,
    )

    expect(screen.getByRole('link', { name: 'Launch Now' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Leave match' })).not.toBeInTheDocument()
  })
})
