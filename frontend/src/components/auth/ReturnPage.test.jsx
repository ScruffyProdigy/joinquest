import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ReturnPage from './ReturnPage'
import * as returnLib from '../../lib/return'
import * as matchResultLib from '../../lib/matchResult'
import {
  REGROUP_ERROR_NOT_FINISHED,
  REGROUP_ERROR_TABLE_FULL,
} from '../../lib/playerCopy'

vi.mock('../../lib/return', () => ({
  fetchReturnDestination: vi.fn(),
}))

vi.mock('../../lib/matchResult', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchMatchResult: vi.fn(),
    playAgain: vi.fn(),
    declinePlayAgain: vi.fn(),
    subscribeToMatchResult: vi.fn(),
  }
})

vi.mock('./AuthProvider', () => ({
  useAuth: () => ({ user: { id: 'a', displayName: 'Ada' } }),
}))

const game = {
  id: 'g1',
  slug: 'word-hunt',
  name: 'Word Hunt',
  modes: [
    { id: 'm1', status: 'active', minPlayers: 2 },
    { id: 'm2', status: 'active', minPlayers: 4 },
  ],
}

function makeResult(overrides = {}) {
  return {
    matchId: 'match-1',
    game,
    reported: true,
    complete: true,
    regroupInviteCode: null,
    participants: [
      { user: { id: 'a', displayName: 'Ada' }, finished: true, placement: 1, winner: true, regroup: 'IN' },
      { user: { id: 'b', displayName: 'Bo' }, finished: true, placement: 2, winner: false, regroup: 'PENDING' },
    ],
    ...overrides,
  }
}

describe('ReturnPage', () => {
  const assign = vi.fn()
  const replaceState = vi.fn()
  const unsubscribe = vi.fn()
  let realReplaceState

  beforeEach(() => {
    vi.mocked(returnLib.fetchReturnDestination).mockReset()
    vi.mocked(matchResultLib.fetchMatchResult).mockReset()
    vi.mocked(matchResultLib.playAgain).mockReset()
    vi.mocked(matchResultLib.declinePlayAgain).mockReset()
    vi.mocked(matchResultLib.subscribeToMatchResult).mockReset()
    assign.mockReset()
    replaceState.mockReset()
    unsubscribe.mockReset()

    vi.mocked(matchResultLib.subscribeToMatchResult).mockResolvedValue(unsubscribe)
    vi.stubGlobal('location', { ...window.location, assign, search: '' })
    realReplaceState = window.history.replaceState
    window.history.replaceState = replaceState
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    window.history.replaceState = realReplaceState
  })

  describe('branch 1: the redirect fallback', () => {
    it('redirects with no match id and never looks up a result', async () => {
      vi.mocked(returnLib.fetchReturnDestination).mockResolvedValue({ path: '/games/word-hunt', kind: 'GAME' })
      render(<ReturnPage />)

      await waitFor(() => expect(assign).toHaveBeenCalledWith('/games/word-hunt'))
      expect(returnLib.fetchReturnDestination).toHaveBeenCalledWith(null)
      expect(matchResultLib.fetchMatchResult).not.toHaveBeenCalled()
    })

    it('still redirects when the result lookup fails', async () => {
      window.location.search = '?match=match-1'
      vi.mocked(returnLib.fetchReturnDestination).mockResolvedValue({ path: '/games/word-hunt', kind: 'GAME' })
      vi.mocked(matchResultLib.fetchMatchResult).mockRejectedValue(new Error('you did not play in this match'))

      render(<ReturnPage />)

      await waitFor(() => expect(assign).toHaveBeenCalledWith('/games/word-hunt'))
      expect(screen.queryByText('you did not play in this match')).not.toBeInTheDocument()
    })

    it('still redirects when the result is null', async () => {
      window.location.search = '?match=match-1'
      vi.mocked(returnLib.fetchReturnDestination).mockResolvedValue({ path: '/', kind: 'HOME' })
      vi.mocked(matchResultLib.fetchMatchResult).mockResolvedValue(null)

      render(<ReturnPage />)
      await waitFor(() => expect(assign).toHaveBeenCalledWith('/'))
    })

    it('keeps today\'s error state when the destination itself cannot be resolved', async () => {
      vi.mocked(returnLib.fetchReturnDestination).mockRejectedValue(new Error('Authentication required'))
      render(<ReturnPage />)

      expect(await screen.findByText('Authentication required')).toBeInTheDocument()
      expect(assign).not.toHaveBeenCalled()
      expect(screen.getByRole('link', { name: 'Continue to JoinQuest' })).toBeInTheDocument()
    })
  })

  describe('branch 2: the match is still running', () => {
    it('shows the others as still playing and leaves the viewer out of that list', async () => {
      window.location.search = '?match=match-1'
      vi.mocked(returnLib.fetchReturnDestination).mockResolvedValue({ path: '/', kind: 'HOME' })
      vi.mocked(matchResultLib.fetchMatchResult).mockResolvedValue(
        makeResult({
          complete: false,
          participants: [
            { user: { id: 'a', displayName: 'Ada' }, finished: true, placement: 1, winner: false, regroup: 'PENDING' },
            { user: { id: 'b', displayName: 'Bo' }, finished: false, placement: null, winner: false, regroup: 'PENDING' },
          ],
        }),
      )

      render(<ReturnPage />)

      expect(await screen.findByText('Still playing')).toBeInTheDocument()
      expect(screen.getByText('Bo')).toBeInTheDocument()
      expect(screen.queryByText('Ada')).not.toBeInTheDocument()
      expect(screen.queryByText('Who’s playing again?')).not.toBeInTheDocument()
      expect(assign).not.toHaveBeenCalled()
    })

    it('flips to the standings when the subscription reports the match finished', async () => {
      window.location.search = '?match=match-1'
      vi.mocked(returnLib.fetchReturnDestination).mockResolvedValue({ path: '/', kind: 'HOME' })
      vi.mocked(matchResultLib.fetchMatchResult).mockResolvedValue(makeResult({ complete: false }))

      let push
      vi.mocked(matchResultLib.subscribeToMatchResult).mockImplementation(async (_id, { onUpdate }) => {
        push = onUpdate
        return unsubscribe
      })

      render(<ReturnPage />)
      await screen.findByText('Still playing')

      await waitFor(() => expect(push).toBeTypeOf('function'))
      await act(async () => {
        push(makeResult({ complete: true }))
      })

      expect(await screen.findByText('Final standings')).toBeInTheDocument()
      expect(screen.getByText('Who’s playing again?')).toBeInTheDocument()
    })
  })

  describe('branch 3: the match is over', () => {
    async function renderComplete(result = makeResult()) {
      window.location.search = '?match=match-1'
      vi.mocked(returnLib.fetchReturnDestination).mockResolvedValue({ path: '/', kind: 'HOME' })
      vi.mocked(matchResultLib.fetchMatchResult).mockResolvedValue(result)
      render(<ReturnPage />)
      await screen.findByText('Who’s playing again?')
    }

    it('shows the standings and the regroup card, and acknowledges the return first', async () => {
      await renderComplete()
      expect(screen.getByText('Final standings')).toBeInTheDocument()
      expect(returnLib.fetchReturnDestination).toHaveBeenCalledWith('match-1')
      expect(assign).not.toHaveBeenCalled()
    })

    it('gates the round on the smallest mode minimum, counting only IN', async () => {
      await renderComplete()
      // Ada is IN, Bo is PENDING; the smallest active mode needs 2.
      expect(screen.getByRole('button', { name: 'Need 1 more to play again' })).toBeDisabled()
    })

    it('unsubscribes on unmount', async () => {
      window.location.search = '?match=match-1'
      vi.mocked(returnLib.fetchReturnDestination).mockResolvedValue({ path: '/', kind: 'HOME' })
      vi.mocked(matchResultLib.fetchMatchResult).mockResolvedValue(makeResult())
      const { unmount } = render(<ReturnPage />)
      await screen.findByText('Who’s playing again?')
      await waitFor(() => expect(matchResultLib.subscribeToMatchResult).toHaveBeenCalled())

      unmount()
      await waitFor(() => expect(unsubscribe).toHaveBeenCalled())
    })

    it('routes to the regroup room when another round starts', async () => {
      const user = userEvent.setup()
      const ready = makeResult({
        participants: makeResult().participants.map((p) => ({ ...p, regroup: 'IN' })),
      })
      await renderComplete(ready)
      vi.mocked(matchResultLib.playAgain).mockResolvedValue({ inviteCode: 'ABC123', seated: true, table: {} })

      await user.click(screen.getByRole('button', { name: 'Another round' }))

      await waitFor(() => expect(assign).toHaveBeenCalledWith('/room/ABC123'))
      expect(matchResultLib.playAgain).toHaveBeenCalledWith('match-1')
    })

    it('falls back to the game page when the match has no mode to rebuild from', async () => {
      const user = userEvent.setup()
      const ready = makeResult({
        participants: makeResult().participants.map((p) => ({ ...p, regroup: 'IN' })),
      })
      await renderComplete(ready)
      vi.mocked(matchResultLib.playAgain).mockRejectedValue(
        new Error('this match no longer has a mode to build a table from'),
      )

      await user.click(screen.getByRole('button', { name: 'Another round' }))

      await waitFor(() => expect(assign).toHaveBeenCalledWith('/games/word-hunt'))
    })

    it('says the match is not finished rather than routing away', async () => {
      const user = userEvent.setup()
      const ready = makeResult({
        participants: makeResult().participants.map((p) => ({ ...p, regroup: 'IN' })),
      })
      await renderComplete(ready)
      vi.mocked(matchResultLib.playAgain).mockRejectedValue(new Error("this match hasn't finished yet"))

      await user.click(screen.getByRole('button', { name: 'Another round' }))

      expect(await screen.findByText(REGROUP_ERROR_NOT_FINISHED)).toBeInTheDocument()
      expect(assign).not.toHaveBeenCalled()
    })

    it('says the table filled up rather than routing away', async () => {
      const user = userEvent.setup()
      const ready = makeResult({
        participants: makeResult().participants.map((p) => ({ ...p, regroup: 'IN' })),
      })
      await renderComplete(ready)
      vi.mocked(matchResultLib.playAgain).mockRejectedValue(new Error('the table is full'))

      await user.click(screen.getByRole('button', { name: 'Another round' }))

      expect(await screen.findByText(REGROUP_ERROR_TABLE_FULL)).toBeInTheDocument()
      expect(assign).not.toHaveBeenCalled()
    })

    it('declines before leaving for something new', async () => {
      const user = userEvent.setup()
      await renderComplete()
      vi.mocked(matchResultLib.declinePlayAgain).mockResolvedValue({ path: '/', kind: 'HOME' })

      await user.click(screen.getByRole('button', { name: 'Find something new' }))

      await waitFor(() => expect(matchResultLib.declinePlayAgain).toHaveBeenCalledWith('match-1'))
      await waitFor(() => expect(assign).toHaveBeenCalledWith('/'))
    })

    it('declines before going back to the game', async () => {
      const user = userEvent.setup()
      await renderComplete()
      vi.mocked(matchResultLib.declinePlayAgain).mockResolvedValue({ path: '/', kind: 'HOME' })

      await user.click(screen.getByRole('button', { name: 'Back to Word Hunt' }))

      await waitFor(() => expect(matchResultLib.declinePlayAgain).toHaveBeenCalledWith('match-1'))
      await waitFor(() => expect(assign).toHaveBeenCalledWith('/games/word-hunt'))
    })

    it('leaves anyway when the decline fails — the player is not trapped', async () => {
      const user = userEvent.setup()
      await renderComplete()
      vi.mocked(matchResultLib.declinePlayAgain).mockRejectedValue(new Error('boom'))

      await user.click(screen.getByRole('button', { name: 'Find something new' }))

      await waitFor(() => expect(assign).toHaveBeenCalledWith('/'))
    })
  })
})
