import { afterEach, describe, expect, it, vi } from 'vitest'
import { matchHeadline, ordinal } from './playerCopy'

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
})

describe('matchHeadline', () => {
  it('celebrates a win', () => {
    expect(matchHeadline({ reason: 'COMPLETED', placement: 1, playerCount: 4 }))
      .toEqual({ headline: '1st place.', sub: 'You finished on top.' })
  })

  it('softens last place', () => {
    expect(matchHeadline({ reason: 'COMPLETED', placement: 4, playerCount: 4 }))
      .toEqual({ headline: '4th place.', sub: 'Better luck next time.' })
  })

  it('reports elimination against the field', () => {
    expect(matchHeadline({ reason: 'ELIMINATED', placement: 5, playerCount: 6 }))
      .toEqual({ headline: '5th of 6.', sub: 'Eliminated before the end.' })
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
