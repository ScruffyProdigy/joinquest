import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import GameQueueActions from './GameQueueActions'

describe('GameQueueActions', () => {
  it('shows a Look for group button for single-bucket modes', () => {
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="idle"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.queryByRole('region', { name: 'Look for group' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Look for group' })).toBeInTheDocument()
  })

  it('shows waiting controls without a Look for group panel in fifo modes', () => {
    render(
      <GameQueueActions
        joinOptions={{ kind: 'fifo', paths: [] }}
        queueState="waiting"
        busy={false}
        onJoin={vi.fn()}
        onLeave={vi.fn()}
      />,
    )

    expect(screen.queryByRole('region', { name: 'Look for group' })).not.toBeInTheDocument()
    expect(screen.getByText('Looking…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Stop looking' })).toBeInTheDocument()
  })

  it('shows role buttons inside a Look for group panel for composition modes', () => {
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

    expect(screen.getByRole('region', { name: 'Look for group' })).toBeInTheDocument()
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

  it('falls back to a plain Look for group button when composition paths are empty', () => {
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

    expect(screen.queryByRole('region', { name: 'Look for group' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Look for group' })).toBeInTheDocument()
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

    expect(screen.getByText('Looking as Damage…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Stop looking' })).toBeInTheDocument()
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

    expect(screen.getByText('Looking as Clue Giver…')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Join as Clue Giver' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join as Guesser' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Join as Guesser' }))
    expect(onJoin).toHaveBeenCalledWith('Guesser')
  })

  it('shows a Play button instead of Look for group for solo modes', () => {
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

    expect(screen.getByRole('button', { name: 'Play' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Look for group' })).not.toBeInTheDocument()
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

  it('calls onJoin with no queue path when Play is clicked in a solo mode', async () => {
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

    await userEvent.click(screen.getByRole('button', { name: 'Play' }))
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

    expect(screen.getByRole('link', { name: 'Launch game' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Leave match' })).not.toBeInTheDocument()
  })
})
