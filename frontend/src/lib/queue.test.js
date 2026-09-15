import { describe, expect, it } from 'vitest'
import { QUEUE_RECONNECT_ATTEMPTS, queueReconnectWaitMs } from './queue'

// The other half of store.DefaultQueueDisconnectGrace, which is pinned by its own Go
// test. The server holds a waiting player's place for exactly as long as this client
// keeps asking for it, so the budget these two constants produce has to land just under
// the server's 90s — under, because a client that stops asking first leaves a rivalrous
// queue place held for a browser that has already given up on it, and the people behind
// it in the queue pay for every second of that (JQ-283).
describe('queue reconnect budget', () => {
  const budgetMs = () =>
    Array.from({ length: QUEUE_RECONNECT_ATTEMPTS }, (_, retries) => queueReconnectWaitMs(retries))
      .reduce((total, wait) => total + wait, 0)

  const QUEUE_DISCONNECT_GRACE_MS = 90 * 1000

  it('keeps asking for just under the server 90 second grace', () => {
    expect(budgetMs()).toBe(87_500)
    expect(budgetMs()).toBeLessThan(QUEUE_DISCONNECT_GRACE_MS)
  })

  // The bug this replaced: 10 attempts is 22.5s, which gave up 67.5s before the server
  // did. Raising the grace window without raising this reopens it, so fail on the gap
  // rather than only on the total.
  it('leaves no window where the server holds a place nobody is asking for', () => {
    expect(QUEUE_DISCONNECT_GRACE_MS - budgetMs()).toBeLessThan(5_000)
  })

  // Reading the count off a 1-based curve is how a previous derivation landed on 27.5s
  // for what was really 22.5s, so pin the 0-based first wait directly.
  it('waits nothing before the first retry, because graphql-ws counts from zero', () => {
    expect(queueReconnectWaitMs(0)).toBe(0)
    expect(queueReconnectWaitMs(10)).toBe(5000)
  })
})
