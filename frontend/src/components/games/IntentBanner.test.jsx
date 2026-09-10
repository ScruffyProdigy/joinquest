import { render, screen } from '@testing-library/react'
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

  it('shows playing intent with launch link and leave game', () => {
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
    expect(screen.getByRole('link', { name: 'Launch Now' })).toHaveAttribute(
      'href',
      'http://game.example/play',
    )
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
    expect(screen.getByRole('link', { name: 'Launch Now' })).toHaveAttribute(
      'href',
      'https://play.example.com/match?token=abc',
    )
  })

  it('shows launch link from table seat when matched intent has no joinUrl', () => {
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
    expect(screen.getByRole('link', { name: 'Launch Now' })).toHaveAttribute(
      'href',
      'https://play.example.com/table?token=xyz',
    )
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
    expect(screen.getByRole('link', { name: 'Launch Now' })).toHaveAttribute(
      'href',
      'https://play.example.com/start',
    )
    expect(screen.getByRole('button', { name: 'Leave game' })).toBeInTheDocument()
  })

  it('shows leave game when matched without a launch link', () => {
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
    expect(screen.queryByRole('link', { name: 'Launch Now' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Leave game' })).toBeInTheDocument()
  })
})
