import { describe, expect, it } from 'vitest'
import { LONG_QUEUE_SECONDS, isLongQueue } from './queueLength'

describe('isLongQueue', () => {
  it('treats a short wait as short', () => {
    // The prompt is spent once and a denial is permanent per origin, so a
    // fifteen-second queue must not cost the player their only ask.
    expect(isLongQueue(15)).toBe(false)
    expect(isLongQueue(LONG_QUEUE_SECONDS - 1)).toBe(false)
  })

  it('treats the threshold itself as long', () => {
    expect(isLongQueue(LONG_QUEUE_SECONDS)).toBe(true)
  })

  it('treats a long wait as long', () => {
    expect(isLongQueue(300)).toBe(true)
  })

  it('treats an unknown wait as long', () => {
    // JQ-58 answers null on a cold queue, which is where waits run longest.
    // Reading that as "short" would withhold the control from the players it
    // is for.
    expect(isLongQueue(null)).toBe(true)
    expect(isLongQueue(undefined)).toBe(true)
  })

  it('does not read zero as unknown', () => {
    expect(isLongQueue(0)).toBe(false)
  })
})
