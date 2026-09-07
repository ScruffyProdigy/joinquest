import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  formatBackToGame,
  formatNeedMorePlayers,
  matchHeadline,
  ordinal,
  REGROUP_ANOTHER_ROUND,
  REGROUP_FIND_SOMETHING_NEW,
  REGROUP_IN,
  REGROUP_OUT,
  REGROUP_PENDING,
  REGROUP_TITLE,
  RESULTS_FINAL_TITLE,
  RESULTS_IN_PROGRESS,
  RESULTS_IN_PROGRESS_TITLE,
  RESULTS_STILL_PLAYING,
  RESULTS_WINNER,
  RESULTS_YOU,
  RESULTS_YOUR_RESULT_SO_FAR,
} from './playerCopy'

vi.mock('./graphql', () => ({
  graphqlRequest: vi.fn(),
}))

describe('ordinal', () => {
  it('handles the teens', () => {
    expect(ordinal(1)).toBe('1st')
    expect(ordinal(2)).toBe('2nd')
    expect(ordinal(3)).toBe('3rd')
    expect(ordinal(11)).toBe('11th')
    expect(ordinal(12)).toBe('12th')
    expect(ordinal(13)).toBe('13th')
    expect(ordinal(21)).toBe('21st')
  })

  it('handles the hundred-teens (11-13 exception repeats every hundred)', () => {
    expect(ordinal(111)).toBe('111th')
    expect(ordinal(112)).toBe('112th')
    expect(ordinal(113)).toBe('113th')
  })
})

describe('matchHeadline', () => {
  it('celebrates a win when the match is complete', () => {
    expect(matchHeadline({ reason: 'COMPLETED', complete: true, placement: 1, playerCount: 4 }))
      .toEqual({ headline: '1st place.', sub: 'You finished on top.' })
  })

  it('softens last place when the match is complete', () => {
    expect(matchHeadline({ reason: 'COMPLETED', complete: true, placement: 4, playerCount: 4 }))
      .toEqual({ headline: '4th place.', sub: 'Better luck next time.' })
  })

  it('reports a mid-pack complete finish as "Out of N players."', () => {
    expect(matchHeadline({ reason: 'COMPLETED', complete: true, placement: 2, playerCount: 4 }))
      .toEqual({ headline: '2nd place.', sub: 'Out of 4 players.' })
  })

  it('reports elimination against the field', () => {
    expect(matchHeadline({ reason: 'ELIMINATED', complete: true, placement: 5, playerCount: 6 }))
      .toEqual({ headline: '5th of 6.', sub: 'Eliminated before the end.' })
  })

  it('tells a player who finished early that others are still playing', () => {
    // The participant's own reason is COMPLETED (they finished their play);
    // it's the match, not the participant, that isn't done yet.
    expect(matchHeadline({ reason: 'COMPLETED', complete: false, placement: 2, playerCount: 4 }))
      .toEqual({ headline: '2nd place.', sub: 'Waiting for others to finish.' })
  })

  it('reports elimination even while the match is still running for others', () => {
    // ELIMINATED is checked ahead of `complete` so the more specific message
    // wins instead of being masked by "Waiting for others to finish."
    expect(matchHeadline({ reason: 'ELIMINATED', complete: false, placement: 5, playerCount: 6 }))
      .toEqual({ headline: '5th of 6.', sub: 'Eliminated before the end.' })
  })
})

// Copy fidelity: these strings were lifted verbatim from the product's
// Figma prototype and must match exactly, including the two non-ASCII
// characters (a typographic apostrophe and an ellipsis) an editor's
// autocorrect could silently "fix".
describe('post-game copy constants', () => {
  it('pins the results panel strings', () => {
    expect(RESULTS_IN_PROGRESS_TITLE).toBe('Results so far')
    expect(RESULTS_FINAL_TITLE).toBe('Final standings')
    expect(RESULTS_WINNER).toBe('Winner')
    expect(RESULTS_STILL_PLAYING).toBe('Still playing')
    expect(RESULTS_YOU).toBe('You')
    expect(RESULTS_YOUR_RESULT_SO_FAR).toBe('Your result so far')
  })

  it('pins RESULTS_IN_PROGRESS, including the ellipsis character (U+2026, not three dots)', () => {
    expect(RESULTS_IN_PROGRESS).toBe('In progress…')
    expect(RESULTS_IN_PROGRESS.codePointAt(RESULTS_IN_PROGRESS.length - 1)).toBe(0x2026)
  })

  it('pins the regroup panel strings', () => {
    expect(REGROUP_IN).toBe('In')
    expect(REGROUP_OUT).toBe('Out')
    expect(REGROUP_PENDING).toBe('Not back yet')
    expect(REGROUP_ANOTHER_ROUND).toBe('Another round')
    expect(REGROUP_FIND_SOMETHING_NEW).toBe('Find something new')
  })

  it('pins REGROUP_TITLE, including the typographic apostrophe (U+2019, not an ASCII quote)', () => {
    expect(REGROUP_TITLE).toBe('Who’s playing again?')
    expect(REGROUP_TITLE.codePointAt(3)).toBe(0x2019)
  })

  it('formats formatBackToGame', () => {
    expect(formatBackToGame('Codenames')).toBe('Back to Codenames')
  })

  it('formats formatNeedMorePlayers', () => {
    expect(formatNeedMorePlayers(2)).toBe('Need 2 more to play again')
  })
})

// Data-layer smoke tests: exercise the request-shaping logic by mocking
// graphqlRequest, the way auth.test.js does. Not covered here: the
// subscription helpers (subscribeToMatchResult), which would need mocking
// the whole graphql-ws client — out of scope for a cheap unit test.
describe('matchResult data layer', () => {
  afterEach(async () => {
    const { graphqlRequest } = await import('./graphql')
    vi.mocked(graphqlRequest).mockReset()
  })

  it('fetchMatchResult passes matchId and unwraps the result', async () => {
    const { graphqlRequest } = await import('./graphql')
    const { fetchMatchResult } = await import('./matchResult')
    const result = { matchId: 'm1', status: 'COMPLETED' }
    vi.mocked(graphqlRequest).mockResolvedValue({ matchResult: result })

    const data = await fetchMatchResult('m1')

    expect(data).toEqual(result)
    expect(graphqlRequest).toHaveBeenCalledWith(expect.stringContaining('matchResult'), { matchId: 'm1' })
  })

  it('playAgain passes matchId and unwraps the result', async () => {
    const { graphqlRequest } = await import('./graphql')
    const { playAgain } = await import('./matchResult')
    const result = { table: { id: 't1' }, inviteCode: 'ABCD', seated: true }
    vi.mocked(graphqlRequest).mockResolvedValue({ playAgain: result })

    const data = await playAgain('m1')

    expect(data).toEqual(result)
    expect(graphqlRequest).toHaveBeenCalledWith(expect.stringContaining('playAgain'), { matchId: 'm1' })
  })

  it('declinePlayAgain passes matchId and unwraps the result', async () => {
    const { graphqlRequest } = await import('./graphql')
    const { declinePlayAgain } = await import('./matchResult')
    const result = { path: '/', kind: 'home' }
    vi.mocked(graphqlRequest).mockResolvedValue({ declinePlayAgain: result })

    const data = await declinePlayAgain('m1')

    expect(data).toEqual(result)
    expect(graphqlRequest).toHaveBeenCalledWith(expect.stringContaining('declinePlayAgain'), { matchId: 'm1' })
  })
})
