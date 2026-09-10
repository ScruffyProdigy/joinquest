import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import IntentBanner from './IntentBanner'
import { LEAVE_GAME_FAILED } from '../../lib/playerCopy'

describe('IntentBanner', () => {
  it('renders nothing without an active intent', () => {
    const { container } = render(
      <IntentBanner activeIntent={null} activeTableSeat={null} busy={false} onLeave={vi.fn()} />,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('shows a leave failure alongside the playing intent', () => {
    render(
      <IntentBanner
        activeIntent={{
          queueId: 'q1',
          gameName: 'Demo Game',
          modeName: 'Duel',
          status: 'MATCHED',
        }}
        activeTableSeat={null}
        busy={false}
        leaveError={LEAVE_GAME_FAILED}
        onLeave={vi.fn()}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent(LEAVE_GAME_FAILED)
  })

  it('renders nothing for a queued intent — the waiting page owns that state', () => {
    const { container } = render(
      <IntentBanner
        activeIntent={{ queueId: 'q1', gameName: 'Demo Game', status: 'WAITING', queuedCount: 3 }}
        activeTableSeat={null}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('shows no alert when leaving has not failed', () => {
    render(
      <IntentBanner
        activeIntent={{
          queueId: 'q1',
          gameName: 'Demo Game',
          modeName: 'Duel',
          status: 'MATCHED',
        }}
        activeTableSeat={null}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('offers rejoin and leave game once a match is live', () => {
    render(
      <IntentBanner
        activeIntent={{
          queueId: 'q1',
          gameName: 'Demo Game',
          status: 'MATCHED',
          joinUrl: 'http://game.example/play',
        }}
        activeTableSeat={null}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(screen.getByText(/Playing Demo Game/)).toBeInTheDocument()
    // The rejoin URL is minted on click, so the banner shows an action rather than
    // a link carrying a token that would be stale by the time it was used (JQ-86).
    expect(screen.getByRole('button', { name: 'Rejoin' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /rejoin|launch/i })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Leave game' })).toBeInTheDocument()
  })

  it('shows table seat intent pointing to room nav', () => {
    render(
      <IntentBanner
        activeIntent={null}
        activeTableSeat={{
          tableId: 't1',
          gameName: 'Word Hunt',
          modeName: 'Word Hunt Party',
          seatDisplayName: 'Guesser · 2',
        }}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(screen.getByText(/Use Room below/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Leave seat' })).toBeInTheDocument()
  })

  it('shows table backfill gaps on the intent banner', () => {
    render(
      <IntentBanner
        activeIntent={null}
        activeTableSeat={{
          tableId: 't1',
          gameName: 'Word Hunt',
          modeName: 'Word Hunt Party',
          seatDisplayName: 'Clue Giver · Red',
          backfillActive: true,
          formingGaps: [
            { queuePath: 'ClueGiver', displayName: 'Clue Giver', assigned: 1, needed: 1 },
            { queuePath: 'Guesser', displayName: 'Guesser', assigned: 1, needed: 3 },
          ],
        }}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(
      screen.getByText('Your table is queued to start — need 1 Clue Giver, 3 Guesser'),
    ).toBeInTheDocument()
  })

  it('shows launch link when playing via myActiveIntent with role', () => {
    render(
      <IntentBanner
        activeIntent={{
          gameName: 'Word Hunt',
          modeName: 'Word Hunt Party',
          seatDisplayName: 'Guesser · 2',
          status: 'MATCHED',
          joinUrl: 'https://play.example.com/match?token=abc',
        }}
        activeTableSeat={null}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(screen.getByText(/Playing Word Hunt \(Word Hunt Party\) as Guesser · 2/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rejoin' })).toBeInTheDocument()
  })

  it('offers rejoin from a table seat when matched intent has no joinUrl', () => {
    render(
      <IntentBanner
        activeIntent={{
          gameName: 'Word Hunt',
          modeName: 'Word Hunt Party',
          status: 'MATCHED',
        }}
        activeTableSeat={{
          status: 'started',
          gameName: 'Word Hunt',
          modeName: 'Word Hunt Party',
          seatDisplayName: 'Guesser · 2',
          joinUrl: 'https://play.example.com/table?token=xyz',
        }}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(screen.getByText(/Playing Word Hunt \(Word Hunt Party\) as Guesser · 2/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rejoin' })).toBeInTheDocument()
  })

  it('shows playing banner for a started table session without catalog intent', () => {
    render(
      <IntentBanner
        activeIntent={null}
        activeTableSeat={{
          status: 'started',
          gameName: 'Demo Game',
          modeName: 'Classic',
          joinUrl: 'https://play.example.com/start',
        }}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(screen.getByText(/Playing Demo Game \(Classic\)/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rejoin' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Leave game' })).toBeInTheDocument()
  })

  it('holds back rejoin until the match has somewhere to send the player', () => {
    render(
      <IntentBanner
        activeIntent={{
          gameName: 'Word Hunt',
          modeName: 'Word Hunt Party',
          status: 'MATCHED',
        }}
        activeTableSeat={null}
        busy={false}
        onLeave={vi.fn()}
      />,
    )
    expect(screen.getByText(/Playing Word Hunt/)).toBeInTheDocument()
    expect(screen.getByText(/Preparing your launch link/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Rejoin' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Leave game' })).toBeInTheDocument()
  })

  it('asks for a fresh way in when rejoin is clicked', async () => {
    const onRejoin = vi.fn()
    render(
      <IntentBanner
        activeIntent={{
          queueId: 'q1',
          gameName: 'Demo Game',
          status: 'MATCHED',
          joinUrl: 'http://game.example/play',
        }}
        activeTableSeat={null}
        busy={false}
        onLeave={vi.fn()}
        onRejoin={onRejoin}
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: 'Rejoin' }))
    expect(onRejoin).toHaveBeenCalledTimes(1)
  })

  it('surfaces a rejoin failure without hiding the leave action', () => {
    render(
      <IntentBanner
        activeIntent={{
          queueId: 'q1',
          gameName: 'Demo Game',
          status: 'MATCHED',
          joinUrl: 'http://game.example/play',
        }}
        activeTableSeat={null}
        busy={false}
        rejoinError="That match has finished, so there is nothing to rejoin."
        onLeave={vi.fn()}
        onRejoin={vi.fn()}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent(/nothing to rejoin/)
    expect(screen.getByRole('button', { name: 'Leave game' })).toBeInTheDocument()
  })

  it('disables rejoin while a rejoin is in flight', () => {
    render(
      <IntentBanner
        activeIntent={{
          queueId: 'q1',
          gameName: 'Demo Game',
          status: 'MATCHED',
          joinUrl: 'http://game.example/play',
        }}
        activeTableSeat={null}
        busy={false}
        rejoining
        onLeave={vi.fn()}
        onRejoin={vi.fn()}
      />,
    )
    expect(screen.getByRole('button', { name: '…' })).toBeDisabled()
  })
})
