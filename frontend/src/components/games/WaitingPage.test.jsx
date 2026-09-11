import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import WaitingPage from './WaitingPage'
import { useLeaveQueueOnExit } from './useLeaveQueueOnExit'
import { LEAVE_GAME_FAILED, NOTIFY_ME } from '../../lib/playerCopy'
import {
  mockAuthenticatedSession,
  mockPushSupported,
  mockPushUnsupported,
} from '../../test/setup'

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
    vi.mocked(useLeaveQueueOnExit).mockClear()
  })

  /** What the hook calls when it has undone a back gesture and needs the player asked. */
  function backGestureHandler() {
    return vi.mocked(useLeaveQueueOnExit).mock.calls.at(-1)?.[1]
  }

  it('shows the queued game, how many are looking, and the way out', () => {
    setIntentState({ activeIntent: waitingIntent })
    render(<WaitingPage intent={intentState} />)

    expect(screen.getByRole('heading', { name: 'Finding players…' })).toBeInTheDocument()
    expect(screen.getByText('Word Hunt · Arena')).toBeInTheDocument()
    expect(screen.getByText('Looking for players… (3 players looking)')).toBeInTheDocument()
    expect(screen.getByText('We will notify you here when your group is ready.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Stop' })).toBeInTheDocument()
  })

  describe('the notify opt-in', () => {
    const PUBLIC_KEY =
      'BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM'

    afterEach(() => {
      mockPushUnsupported()
      vi.restoreAllMocks()
    })

    it('offers it on a long queue', async () => {
      mockPushSupported()
      mockAuthenticatedSession()
      setIntentState({ activeIntent: { ...waitingIntent, estimatedWaitSeconds: 240 } })
      render(<WaitingPage intent={intentState} />)

      expect(await screen.findByRole('button', { name: NOTIFY_ME })).toBeInTheDocument()
    })

    it('withholds it on a short queue', async () => {
      mockPushSupported()
      mockAuthenticatedSession()
      setIntentState({ activeIntent: { ...waitingIntent, estimatedWaitSeconds: 15 } })
      render(<WaitingPage intent={intentState} />)

      // The queue pops before leaving the page is worth doing, and the
      // permission prompt only comes round once.
      await waitFor(() => expect(screen.getByRole('button', { name: 'Stop' })).toBeInTheDocument())
      expect(screen.queryByRole('button', { name: NOTIFY_ME })).not.toBeInTheDocument()
    })

    it('changes what the page says once the opt-in is in force', async () => {
      mockPushSupported()
      mockAuthenticatedSession(undefined, {
        pushCapability: { reachable: true, subscriptionCount: 1, publicKey: PUBLIC_KEY },
      })
      setIntentState({ activeIntent: { ...waitingIntent, estimatedWaitSeconds: 240 } })
      render(<WaitingPage intent={intentState} />)

      // A page that still says "we will notify you here" is not permission to
      // stop looking at it.
      expect(
        await screen.findByText("Put your phone away — we'll notify you when your group is ready."),
      ).toBeInTheDocument()
      expect(
        screen.queryByText('We will notify you here when your group is ready.'),
      ).not.toBeInTheDocument()
    })
  })

  it('shows how much longer the player is likely to wait', () => {
    setIntentState({ activeIntent: { ...waitingIntent, estimatedWaitSeconds: 90 } })
    render(<WaitingPage intent={intentState} />)

    expect(screen.getByText('About 2 min left')).toBeInTheDocument()
  })

  // No estimate is no line, not a hedge: the backend already declined to guess.
  it('shows no wait line when there is no estimate', () => {
    setIntentState({ activeIntent: { ...waitingIntent, estimatedWaitSeconds: null } })
    render(<WaitingPage intent={intentState} />)

    expect(screen.queryByText(/left$/)).not.toBeInTheDocument()
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

    await user.click(screen.getByRole('button', { name: 'Stop' }))
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

    await user.click(screen.getByRole('button', { name: 'Stop' }))
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

    // The fetch lands and reports no intent at all: the player left the queue.
    setIntentState({ loading: false })
    rerender(<WaitingPage intent={intentState} />)

    await waitFor(() => expect(window.location.pathname).toBe('/games/word-hunt'))
  })

  it('hands a formed match to the launch step instead of routing away', async () => {
    setIntentState({ activeIntent: { ...waitingIntent, status: 'MATCHED', joinUrl: null } })
    render(<WaitingPage intent={intentState} />)

    expect(screen.getByRole('heading', { name: "You're in!" })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Finding players…' })).toBeNull()
    await waitFor(() => expect(window.location.pathname).toBe('/waiting'))
  })

  it('sends a signed-out visitor away without waiting on a fetch', async () => {
    authState.user = null
    setIntentState()
    render(<WaitingPage intent={intentState} />)

    await waitFor(() => expect(window.location.pathname).toBe('/'))
  })

  it('routes away when a queued intent it was already showing disappears', async () => {
    // Distinct from the case above, which never got past loading: this one has
    // rendered the queued state before the intent goes, so it pins that the page
    // gives up a view it is already committed to rather than only declining to
    // enter it.
    //
    // That is the transition a LEFT update produces — the server evicted a frozen
    // tab, the reconnect's initial payload says LEFT, and useActiveIntent's refresh
    // settles to no intent — but the LEFT payload itself is upstream of this
    // component and is handled in useGameQueue, so nothing here mocks one.
    sessionStorage.setItem('lobby.waitingReturnPath', '/games/word-hunt')
    setIntentState({ loading: true })
    const { rerender } = render(<WaitingPage intent={intentState} />)
    expect(window.location.pathname).toBe('/waiting')

    setIntentState({ activeIntent: waitingIntent })
    rerender(<WaitingPage intent={intentState} />)
    expect(screen.getByRole('heading', { name: 'Finding players…' })).toBeInTheDocument()

    setIntentState({ activeIntent: null })
    rerender(<WaitingPage intent={intentState} />)

    await waitFor(() => expect(window.location.pathname).toBe('/games/word-hunt'))
    expect(screen.queryByRole('heading', { name: 'Finding players…' })).toBeNull()
  })

  it('raises the same confirmation for a back gesture as for the button', async () => {
    setIntentState({ activeIntent: waitingIntent })
    render(<WaitingPage intent={intentState} />)

    act(() => backGestureHandler()())

    expect(await screen.findByText('Leave the queue?')).toBeInTheDocument()
    expect(screen.getByText("You'll lose your spot and have to start over.")).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Stay in queue' })).toBeInTheDocument()
  })

  it('gives up the queue only once the back-gesture confirmation is accepted', async () => {
    const handleLeave = vi.fn()
    setIntentState({ activeIntent: waitingIntent, handleLeave })
    const user = userEvent.setup()
    render(<WaitingPage intent={intentState} />)

    act(() => backGestureHandler()())
    expect(handleLeave).not.toHaveBeenCalled()

    await user.click(await screen.findByRole('button', { name: 'Leave queue' }))

    expect(handleLeave).toHaveBeenCalled()
  })
})
