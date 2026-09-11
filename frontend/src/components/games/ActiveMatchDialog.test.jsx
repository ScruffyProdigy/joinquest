import { render, screen, act, fireEvent } from '@testing-library/react'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import ActiveMatchDialog from './ActiveMatchDialog'
import { LAUNCH_COUNTDOWN_SECONDS } from '../../lib/launch'
import { LEAVE_GAME, MATCH_IN_PROGRESS, REJOIN_MATCH, REJOIN_OVER } from '../../lib/playerCopy'

const liveIntent = {
  queueId: 'q1',
  gameId: 'g1',
  gameName: 'Word Hunt',
  modeName: 'Arena',
  seatDisplayName: 'Clue Giver',
  status: 'MATCHED',
  joinUrl: 'https://game.example/play?match=m1',
}

const formingSeat = {
  tableId: 't1',
  status: 'forming',
  gameName: 'Word Hunt',
  modeName: 'Arena',
  seatDisplayName: 'Clue Giver',
}

function setVisibility(state) {
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    get: () => state,
  })
}

function renderDialog(overrides = {}) {
  const props = {
    activeIntent: liveIntent,
    activeTableSeat: null,
    busy: false,
    leaveError: null,
    rejoinError: null,
    rejoining: false,
    onLeave: vi.fn(),
    onRejoin: vi.fn(),
    ...overrides,
  }
  return { props, ...render(<ActiveMatchDialog {...props} />) }
}

function runCountdown() {
  act(() => {
    vi.advanceTimersByTime(LAUNCH_COUNTDOWN_SECONDS * 1000)
  })
}

describe('ActiveMatchDialog', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setVisibility('visible')
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('tells a player with a live match what they are in, and names the seat', () => {
    renderDialog()

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: MATCH_IN_PROGRESS })).toBeInTheDocument()
    expect(screen.getByText('Word Hunt · Arena · Clue Giver')).toBeInTheDocument()
  })

  it('goes straight to the countdown, because a game already running is not news', () => {
    renderDialog()

    expect(screen.getByText(`Entering in ${LAUNCH_COUNTDOWN_SECONDS}…`)).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: "You're in!" })).toBeNull()
  })

  it('mints the way back in when the countdown fires, rather than following a URL', () => {
    const onRejoin = vi.fn()
    renderDialog({ onRejoin })

    expect(onRejoin).not.toHaveBeenCalled()
    runCountdown()

    expect(onRejoin).toHaveBeenCalledTimes(1)
  })

  it('offers a manual way in that mints on click', () => {
    const onRejoin = vi.fn()
    renderDialog({ onRejoin })

    const rejoin = screen.getByRole('button', { name: REJOIN_MATCH })
    // A link would carry a token that is stale by the time it is followed (JQ-86).
    expect(rejoin.tagName).toBe('BUTTON')
    expect(rejoin).not.toHaveAttribute('href')

    fireEvent.click(rejoin)
    expect(onRejoin).toHaveBeenCalledTimes(1)
  })

  it('lets the player abandon the match', () => {
    const onLeave = vi.fn()
    renderDialog({ onLeave })

    fireEvent.click(screen.getByRole('button', { name: LEAVE_GAME }))
    expect(onLeave).toHaveBeenCalledTimes(1)
  })

  it('stops carrying a player who chose to leave', () => {
    const onRejoin = vi.fn()
    renderDialog({ onRejoin, onLeave: vi.fn() })

    fireEvent.click(screen.getByRole('button', { name: LEAVE_GAME }))
    runCountdown()

    expect(onRejoin).not.toHaveBeenCalled()
  })

  it('will not be dismissed, because there is nowhere for the player to be dismissed to', () => {
    renderDialog()

    expect(screen.queryByRole('button', { name: /close/i })).toBeNull()

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape', code: 'Escape' })
    act(() => {
      vi.advanceTimersByTime(500)
    })

    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  /*
    The server refuses once the match is over. `handleRejoin` refreshes on that
    failure, so the intent clears and this surface goes with it rather than sitting
    there offering a dead action.
  */
  it('clears itself when the refusal has cleared the intent', () => {
    const { rerender, props } = renderDialog()
    runCountdown()

    rerender(<ActiveMatchDialog {...props} rejoinError={REJOIN_OVER} />)
    expect(screen.getByRole('alert')).toHaveTextContent(REJOIN_OVER)

    rerender(<ActiveMatchDialog {...props} activeIntent={null} rejoinError={REJOIN_OVER} />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('does not fire a second doomed rejoin after the server refused the first', () => {
    const onRejoin = vi.fn()
    const { rerender, props } = renderDialog({ onRejoin })
    runCountdown()
    expect(onRejoin).toHaveBeenCalledTimes(1)

    // A refusal leaves this mounted and still armed at zero. Without a latch every
    // re-render would fire another attempt.
    rerender(<ActiveMatchDialog {...props} onRejoin={onRejoin} rejoinError={REJOIN_OVER} />)
    runCountdown()

    expect(onRejoin).toHaveBeenCalledTimes(1)
  })

  it('holds rather than yanking a tab the player is not looking at', () => {
    setVisibility('hidden')
    const onRejoin = vi.fn()
    renderDialog({ onRejoin })

    runCountdown()

    expect(onRejoin).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: REJOIN_MATCH })).toBeInTheDocument()
  })

  it('waits for a match that is still being provisioned instead of counting to a dead end', () => {
    const onRejoin = vi.fn()
    renderDialog({ onRejoin, activeIntent: { ...liveIntent, joinUrl: null } })

    runCountdown()

    expect(onRejoin).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: REJOIN_MATCH })).toBeNull()
  })

  it('reads a started table seat as a live match too', () => {
    renderDialog({
      activeIntent: null,
      activeTableSeat: { ...formingSeat, status: 'started', joinUrl: 'https://game.example/table' },
    })

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: MATCH_IN_PROGRESS })).toBeInTheDocument()
  })

  it('stays out of the way when there is no match at all', () => {
    renderDialog({ activeIntent: null, activeTableSeat: null })

    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('stays out of the way for a queued player, who has a page of their own', () => {
    renderDialog({ activeIntent: { ...liveIntent, status: 'WAITING', joinUrl: null } })

    expect(screen.queryByRole('dialog')).toBeNull()
  })

  /*
    A seat at a table that has not started is not a live match, and blocking someone
    who is still assembling a group would be wrong. The group screen owns that state
    -- it shows the table, the seat and the way out of both (JQ-261).
  */
  it('stays out of the way for a seat at a table that has not started', () => {
    renderDialog({ activeIntent: null, activeTableSeat: formingSeat })

    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('surfaces a leave failure without letting go of the player', () => {
    renderDialog({ leaveError: 'Could not leave right now. Please try again.' })

    expect(screen.getByRole('alert')).toHaveTextContent('Could not leave right now.')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })
})
