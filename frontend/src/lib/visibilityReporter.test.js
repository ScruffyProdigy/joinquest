import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

vi.mock('./graphql', () => ({ graphqlRequest: vi.fn(() => Promise.resolve({})) }))

import { graphqlRequest } from './graphql'
import { startVisibilityReporting, __resetVisibilityReporterForTests } from './visibilityReporter'

function setVisibility(state) {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: state })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('visibility reporting', () => {
  beforeEach(() => {
    __resetVisibilityReporterForTests()
    graphqlRequest.mockClear()
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' })
  })

  afterEach(() => {
    __resetVisibilityReporterForTests()
    vi.restoreAllMocks()
  })

  it('reports hidden when the tab goes to the background', async () => {
    startVisibilityReporting()
    graphqlRequest.mockClear()

    setVisibility('hidden')
    await vi.waitFor(() => expect(graphqlRequest).toHaveBeenCalled())

    const [, variables] = graphqlRequest.mock.calls.at(-1)
    expect(variables.visible).toBe(false)
  })

  it('reports visible again when the player comes back', async () => {
    startVisibilityReporting()
    setVisibility('hidden')
    await vi.waitFor(() => expect(graphqlRequest).toHaveBeenCalled())
    graphqlRequest.mockClear()

    setVisibility('visible')
    await vi.waitFor(() => expect(graphqlRequest).toHaveBeenCalled())

    const [, variables] = graphqlRequest.mock.calls.at(-1)
    expect(variables.visible).toBe(true)
  })

  // Every report from one page load carries the same document id, so the server
  // replaces that document's answer rather than accumulating a row per flip.
  it('uses one stable document id for the life of the page', async () => {
    startVisibilityReporting()
    setVisibility('hidden')
    await vi.waitFor(() => expect(graphqlRequest).toHaveBeenCalled())
    setVisibility('visible')
    await vi.waitFor(() => expect(graphqlRequest).toHaveBeenCalledTimes(2))

    const ids = graphqlRequest.mock.calls.map(([, variables]) => variables.documentId)
    expect(new Set(ids).size).toBe(1)
    expect(ids[0]).toBeTruthy()
  })

  // Reporting is best-effort telemetry about attention. A failed report must not
  // surface to the player or break the page -- not reporting simply reads as
  // present, which is the behaviour that shipped before this existed.
  it('swallows a failed report', async () => {
    graphqlRequest.mockRejectedValueOnce(new Error('offline'))
    startVisibilityReporting()

    expect(() => setVisibility('hidden')).not.toThrow()
    await vi.waitFor(() => expect(graphqlRequest).toHaveBeenCalled())
  })

  // Starting twice must not double-report: the module is imported by several
  // entry points and a second listener would send every change twice.
  it('is idempotent across repeated starts', async () => {
    startVisibilityReporting()
    startVisibilityReporting()
    graphqlRequest.mockClear()

    setVisibility('hidden')
    await vi.waitFor(() => expect(graphqlRequest).toHaveBeenCalled())
    expect(graphqlRequest).toHaveBeenCalledTimes(1)
  })
})
