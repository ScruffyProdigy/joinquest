import { describe, it, expect, beforeEach } from 'vitest'
import { rememberGroupIntent, takeGroupIntentFor } from './pendingGroupIntent'

describe('pendingGroupIntent', () => {
  beforeEach(() => {
    window.sessionStorage.clear()
  })

  it('hands the intent back to the mode it was remembered for', () => {
    rememberGroupIntent('game-1', 'mode-1')

    expect(takeGroupIntentFor('game-1', 'mode-1')).toBe(true)
  })

  it('is consumed once, so the action cannot fire twice', () => {
    rememberGroupIntent('game-1', 'mode-1')
    takeGroupIntentFor('game-1', 'mode-1')

    expect(takeGroupIntentFor('game-1', 'mode-1')).toBe(false)
  })

  it('does not hand a mode row an intent belonging to another mode', () => {
    rememberGroupIntent('game-1', 'mode-1')

    expect(takeGroupIntentFor('game-1', 'mode-2')).toBe(false)
    expect(takeGroupIntentFor('game-2', 'mode-1')).toBe(false)
    // still pending for its real owner
    expect(takeGroupIntentFor('game-1', 'mode-1')).toBe(true)
  })

  it('reports nothing pending when none was remembered', () => {
    expect(takeGroupIntentFor('game-1', 'mode-1')).toBe(false)
  })
})
