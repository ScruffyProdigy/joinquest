import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import WaitingPage from './WaitingPage'
import { LEAVE_GAME_FAILED } from '../../lib/playerCopy'

const authState = { user: { id: 'u1' }, loading: false }
const intentState = {}

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => authState,
}))

vi.mock('./useLeaveQueueOnExit', () => ({
  useLeaveQueueOnExit: vi.fn(),
}))

function setIntentState(overrides = {}) {
  Object.assign(intentState, {
    activeIntent: null,
    loading: false,
    busy: false,
    queueWsConnected: true,
    leaveError: null,
    handleLeave: vi.fn(),
    ...overrides,
  })
}

const waitingIntent = {
  queueId: 'q1',
  gameId: 'g1',
  gameName: 'Word Hunt',
  modeName: 'Arena',
  status: 'WAITING',
  queuedCount: 3,
}

describe('WaitingPage', () => {
  beforeEach(() => {
    sessionStorage.clear()
    window.history.replaceState(null, '', '/waiting')
    authState.user = { id: 'u1' }
    authState.loading = false
    setIntentState()
  })

  it('shows the queued game, how many are looking, and the way out', () => {
    setIntentState({ activeIntent: waitingIntent })
    render(<WaitingPage intent={intentState} />)

    expect(screen.getByRole('heading', { name: 'Finding players…' })).toBeInTheDocument()
    expect(screen.getByText('Word Hunt · Arena')).toBeInTheDocument()
    expect(screen.getByText('Looking for players… (3 players looking)')).toBeInTheDocument()
    expect(screen.getByText('We will notify you here when your group is ready.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Stop looking' })).toBeInTheDocument()
  })

  it('names the cohort and the roles still missing in composition games', () => {
    setIntentState({
      activeIntent: {
        ...waitingIntent,
        queuedCount: 2,
        queuePath: 'ClueGiver',
        queuePathDisplayName: 'Clue Giver',
        formingGaps: [
          { queuePath: 'ClueGiver', displayName: 'Clue Giver', assigned: 1, needed: 1 },
          { queuePath: 'Guesser', displayName: 'Guesser', assigned: 0, needed: 4 },
        ],
      },
    })
    render(<WaitingPage intent={intentState} />)

    expect(screen.getByText('Word Hunt · Arena · Clue Giver')).toBeInTheDocument()
    expect(screen.getByText('Need 1 Clue Giver, 4 Guesser')).toBeInTheDocument()
  })

  it('carries the pre-queue picks into the subline', () => {
    setIntentState({
      activeIntent: {
        ...waitingIntent,
        queuePathDisplayName: 'Clue Giver',
        selectedOptions: [{ labels: ['Hard mode'] }, { labels: ['No timer'] }],
      },
    })
    render(<WaitingPage intent={intentState} />)

    expect(
      screen.getByText('Word Hunt · Arena · Clue Giver · Hard mode · No timer'),
    ).toBeInTheDocument()
  })

  it('says live updates are paused when the queue socket is down', () => {
    setIntentState({ activeIntent: waitingIntent, queueWsConnected: false })
    render(<WaitingPage intent={intentState} />)

    expect(screen.getByText('Live updates paused — refreshing every few seconds.')).toBeInTheDocument()
  })

  it('surfaces a leave failure without routing away', () => {
    setIntentState({ activeIntent: waitingIntent, leaveError: LEAVE_GAME_FAILED })
    render(<WaitingPage intent={intentState} />)

    expect(screen.getByRole('alert')).toHaveTextContent(LEAVE_GAME_FAILED)
    expect(window.location.pathname).toBe('/waiting')
  })

  it('confirms before giving up the place in the queue', async () => {
    const handleLeave = vi.fn()
    setIntentState({ activeIntent: waitingIntent, handleLeave })
    const user = userEvent.setup()
    render(<WaitingPage intent={intentState} />)

    await user.click(screen.getByRole('button', { name: 'Stop looking' }))
    expect(handleLeave).not.toHaveBeenCalled()
    expect(await screen.findByText('Leave the queue?')).toBeInTheDocument()
    expect(
      screen.getByText("You'll lose your spot and have to start over."),
    ).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Leave queue' }))
    expect(handleLeave).toHaveBeenCalledTimes(1)
  })

  it('stays in the queue when the confirmation is declined', async () => {
    const handleLeave = vi.fn()
    setIntentState({ activeIntent: waitingIntent, handleLeave })
    const user = userEvent.setup()
    render(<WaitingPage intent={intentState} />)

    await user.click(screen.getByRole('button', { name: 'Stop looking' }))
    await user.click(await screen.findByRole('button', { name: 'Stay in queue' }))

    expect(handleLeave).not.toHaveBeenCalled()
    expect(screen.queryByText('Leave the queue?')).toBeNull()
  })

  it('waits for the first fetch rather than bouncing a reload straight off', () => {
    setIntentState({ loading: true })
    render(<WaitingPage intent={intentState} />)

    expect(screen.getByRole('status')).toHaveTextContent('Checking your queue…')
    expect(window.location.pathname).toBe('/waiting')
  })

  it('routes back to where the player came from once the queued intent is gone', async () => {
    sessionStorage.setItem('lobby.waitingReturnPath', '/games/word-hunt')
    setIntentState({ loading: true })
    const { rerender } = render(<WaitingPage intent={intentState} />)
    expect(window.location.pathname).toBe('/waiting')

    // The fetch lands and reports no queued intent: left the queue, or matched.
    setIntentState({ loading: false })
    rerender(<WaitingPage intent={intentState} />)

    await waitFor(() => expect(window.location.pathname).toBe('/games/word-hunt'))
  })

  it('sends a signed-out visitor away without waiting on a fetch', async () => {
    authState.user = null
    setIntentState()
    render(<WaitingPage intent={intentState} />)

    await waitFor(() => expect(window.location.pathname).toBe('/'))
  })
})
