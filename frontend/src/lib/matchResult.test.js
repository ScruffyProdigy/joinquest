import { afterEach, describe, expect, it, vi } from 'vitest'
import { classifyRegroupError, REGROUP_ERROR } from './matchResult'
import {
  formatBackToGame,
  formatRegroupInCount,
  matchHeadline,
  ordinal,
  REGROUP_ANOTHER_ROUND,
  REGROUP_BACK_TO_TABLE,
  REGROUP_FIND_SOMETHING_NEW,
  REGROUP_IN,
  REGROUP_OUT,
  REGROUP_PENDING,
  MASCOT_LABEL,
  MATCH_MOOD,
  REGROUP_TITLE,
  RESULTS_FINAL_TITLE,
  RESULTS_IN_PROGRESS,
  RESULTS_IN_PROGRESS_TITLE,
  RESULTS_PLACEMENT_UNKNOWN,
  RESULTS_STILL_PLAYING,
  RESULTS_WINNER,
  RESULTS_YOU,
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
      .toEqual({ headline: '1st place.', sub: 'You finished on top.', mood: MATCH_MOOD.WIN })
  })

  it('softens last place when the match is complete', () => {
    expect(matchHeadline({ reason: 'COMPLETED', complete: true, placement: 4, playerCount: 4 }))
      .toEqual({ headline: '4th place.', sub: 'Better luck next time.', mood: MATCH_MOOD.LOSE })
  })

  it('reports a mid-pack complete finish as "Out of N players."', () => {
    expect(matchHeadline({ reason: 'COMPLETED', complete: true, placement: 2, playerCount: 4 }))
      .toEqual({ headline: '2nd place.', sub: 'Out of 4 players.', mood: MATCH_MOOD.CLOSE })
  })

  it('reports elimination against the field', () => {
    expect(matchHeadline({ reason: 'ELIMINATED', complete: true, placement: 5, playerCount: 6 }))
      .toEqual({ headline: '5th of 6.', sub: 'Eliminated before the end.', mood: MATCH_MOOD.LOSE })
  })

  it('tells a player who finished early that others are still playing', () => {
    // The participant's own reason is COMPLETED (they finished their play);
    // it's the match, not the participant, that isn't done yet.
    expect(matchHeadline({ reason: 'COMPLETED', complete: false, placement: 2, playerCount: 4 }))
      .toEqual({ headline: '2nd place.', sub: 'Waiting for others to finish.', mood: MATCH_MOOD.CLOSE })
  })

  it('reports elimination even while the match is still running for others', () => {
    // ELIMINATED is checked ahead of `complete` so the more specific message
    // wins instead of being masked by "Waiting for others to finish."
    expect(matchHeadline({ reason: 'ELIMINATED', complete: false, placement: 5, playerCount: 6 }))
      .toEqual({ headline: '5th of 6.', sub: 'Eliminated before the end.', mood: MATCH_MOOD.LOSE })
  })

  /**
   * The mood is what the mascot panel above the headline is tinted by (JQ-277), so it has to
   * track the words rather than the raw placement. Leading a race that is still running is
   * the case that makes the point: 1st with the match unfinished is good news, and everything
   * else about waiting is not.
   */
  it('calls leading an unfinished match a win, and any other wait tense', () => {
    expect(matchHeadline({ reason: 'COMPLETED', complete: false, placement: 1, playerCount: 4 }).mood)
      .toBe(MATCH_MOOD.WIN)
    expect(matchHeadline({ reason: 'COMPLETED', complete: false, placement: 3, playerCount: 4 }).mood)
      .toBe(MATCH_MOOD.CLOSE)
  })

  // A forfeit or a drop is not an elimination, so it keeps the ordinary complete-match copy
  // and its mood. Only the roster treats them alike (see src/lib/regroup.js).
  it('does not borrow the elimination copy for a forfeit', () => {
    expect(matchHeadline({ reason: 'FORFEIT', complete: true, placement: 3, playerCount: 4 }))
      .toEqual({ headline: '3rd place.', sub: 'Out of 4 players.', mood: MATCH_MOOD.CLOSE })
  })

  // placement is nullable on MatchParticipantResult, and ordinal(null) is "nullth".
  // Every branch below would otherwise put that in front of the player.
  it('never builds an ordinal from a missing placement', () => {
    expect(matchHeadline({ reason: null, complete: false, placement: null, playerCount: 4 }))
      .toEqual({ headline: RESULTS_PLACEMENT_UNKNOWN, sub: 'Waiting for others to finish.', mood: MATCH_MOOD.CLOSE })
    expect(matchHeadline({ reason: 'ELIMINATED', complete: false, placement: null, playerCount: 4 }))
      .toEqual({ headline: RESULTS_PLACEMENT_UNKNOWN, sub: 'Eliminated before the end.', mood: MATCH_MOOD.LOSE })
    expect(matchHeadline({ reason: 'COMPLETED', complete: true, placement: null, playerCount: 4 }))
      .toEqual({ headline: RESULTS_PLACEMENT_UNKNOWN, sub: 'Out of 4 players.', mood: MATCH_MOOD.CLOSE })
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
  })

  it('pins RESULTS_IN_PROGRESS, including the ellipsis character (U+2026, not three dots)', () => {
    expect(RESULTS_IN_PROGRESS).toBe('In progress…')
    expect(RESULTS_IN_PROGRESS.codePointAt(RESULTS_IN_PROGRESS.length - 1)).toBe(0x2026)
  })

  it('pins the mascot placeholder labels the prototype ships', () => {
    expect(MASCOT_LABEL[MATCH_MOOD.WIN]).toBe('Mascot illustration · celebrating')
    expect(MASCOT_LABEL[MATCH_MOOD.LOSE]).toBe('Mascot illustration · sympathetic')
    expect(MASCOT_LABEL[MATCH_MOOD.CLOSE]).toBe('Mascot illustration · tense')
  })

  it('pins the regroup panel strings', () => {
    expect(REGROUP_IN).toBe('In')
    expect(REGROUP_OUT).toBe('Out')
    expect(REGROUP_PENDING).toBe('Not back yet')
    expect(REGROUP_ANOTHER_ROUND).toBe('Another round')
    expect(REGROUP_BACK_TO_TABLE).toBe('Back to the table')
    expect(REGROUP_FIND_SOMETHING_NEW).toBe('Find something new')
  })

  it('pins REGROUP_TITLE, including the typographic apostrophe (U+2019, not an ASCII quote)', () => {
    expect(REGROUP_TITLE).toBe('Who’s playing again?')
    expect(REGROUP_TITLE.codePointAt(3)).toBe(0x2019)
  })

  it('formats formatBackToGame', () => {
    expect(formatBackToGame('Codenames')).toBe('Back to Codenames')
  })

  // The count is reported, not enforced - see RegroupCard. It names the roster total so a
  // player can tell whether anyone else is still coming.
  it('formats formatRegroupInCount', () => {
    expect(formatRegroupInCount(1, 4, 2)).toBe('1 of 4 back and in \u00b7 needs 2 to start')
  })

  it('drops the start clause when there is no real minimum to name', () => {
    expect(formatRegroupInCount(0, 2, null)).toBe('0 of 2 back and in')
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

describe('classifyRegroupError', () => {
  /** A thrown error as graphqlRequest builds one: message plus the server's extensions.code. */
  function coded(code, message = 'some server sentence') {
    const error = new Error(message)
    error.code = code
    return error
  }

  it('recognises a session with no mode to rebuild from', () => {
    expect(classifyRegroupError(coded('NO_REGROUP_MODE'))).toBe(REGROUP_ERROR.NO_MODE)
  })

  it('recognises a match that is still running', () => {
    expect(classifyRegroupError(coded('SESSION_NOT_FINISHED'))).toBe(REGROUP_ERROR.NOT_FINISHED)
  })

  it('recognises a full table', () => {
    expect(classifyRegroupError(coded('TABLE_FULL'))).toBe(REGROUP_ERROR.TABLE_FULL)
  })

  // The point of JQ-176: the message is copy, and rewording it must not move the player
  // onto a different branch.
  it('ignores the message text entirely', () => {
    expect(classifyRegroupError(coded('TABLE_FULL', 'every seat is taken'))).toBe(REGROUP_ERROR.TABLE_FULL)
    expect(classifyRegroupError(new Error('the table is full'))).toBe(REGROUP_ERROR.UNKNOWN)
    expect(classifyRegroupError(new Error("this match hasn't finished yet"))).toBe(REGROUP_ERROR.UNKNOWN)
  })

  it('treats an uncoded failure as unknown', () => {
    expect(classifyRegroupError(new Error('could not start another round'))).toBe(REGROUP_ERROR.UNKNOWN)
    expect(classifyRegroupError(coded('SOMETHING_ELSE'))).toBe(REGROUP_ERROR.UNKNOWN)
  })

  it('survives a missing or malformed error', () => {
    expect(classifyRegroupError(null)).toBe(REGROUP_ERROR.UNKNOWN)
    expect(classifyRegroupError({})).toBe(REGROUP_ERROR.UNKNOWN)
    expect(classifyRegroupError('a bare string')).toBe(REGROUP_ERROR.UNKNOWN)
  })
})
