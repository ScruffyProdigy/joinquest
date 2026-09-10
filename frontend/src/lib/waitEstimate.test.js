import { describe, it, expect } from 'vitest'
import { estimatedWaitLine } from './playerCopy'

describe('estimatedWaitLine', () => {
  it('reads in seconds under a minute', () => {
    expect(estimatedWaitLine(15)).toBe('About 15 sec left')
  })

  it('reads in minutes at a minute and over', () => {
    expect(estimatedWaitLine(60)).toBe('About 1 min left')
    expect(estimatedWaitLine(150)).toBe('About 3 min left')
  })

  // The backend says null whenever it has nothing honest to report — too little
  // throughput, no history, or a number too large to be useful. The player sees
  // no line at all rather than a hedge.
  it('has no line when there is no estimate', () => {
    expect(estimatedWaitLine(null)).toBeNull()
    expect(estimatedWaitLine(undefined)).toBeNull()
  })

  // A queue that is about to pop should not read as an error or as "0 sec".
  it('floors at a second rather than reading zero', () => {
    expect(estimatedWaitLine(0)).toBe('About 1 sec left')
  })
})
