import { describe, expect, it } from 'vitest'
import {
  ROOM_RECONNECT_ATTEMPTS,
  parseRoomInviteCode,
  roomReconnectWaitMs,
  roomShareText,
} from './rooms'

describe('parseRoomInviteCode', () => {
  it('extracts code from room path', () => {
    expect(parseRoomInviteCode('/room/abc123')).toBe('ABC123')
    expect(parseRoomInviteCode('/room/X7K2M9/')).toBe('X7K2M9')
  })

  it('returns null for non-room paths', () => {
    expect(parseRoomInviteCode('/')).toBeNull()
    expect(parseRoomInviteCode('/return')).toBeNull()
  })
})

describe('roomShareText', () => {
  it('includes join url', () => {
    expect(roomShareText('https://joinquest.cc/room/ABC123')).toContain('https://joinquest.cc/room/ABC123')
  })
})

// The other half of store.DefaultRoomDisconnectGrace, which is pinned by its own Go
// test. The server holds a member's place for exactly as long as this client keeps
// asking for it, so the budget these two constants produce has to land just under the
// server's 5 minutes — under, because a client that stops asking first leaves the room
// held open for a browser that has already given up on it.
describe('room reconnect budget', () => {
  const budgetMs = () =>
    Array.from({ length: ROOM_RECONNECT_ATTEMPTS }, (_, retries) => roomReconnectWaitMs(retries))
      .reduce((total, wait) => total + wait, 0)

  it('keeps asking for just under the server 5 minute grace', () => {
    expect(budgetMs()).toBe(297_500)
    expect(budgetMs()).toBeLessThan(5 * 60 * 1000)
  })

  // Reading the count off a 1-based curve is how the previous derivation landed on
  // 27.5s for what was really 22.5s, so pin the 0-based first wait directly.
  it('waits nothing before the first retry, because graphql-ws counts from zero', () => {
    expect(roomReconnectWaitMs(0)).toBe(0)
    expect(roomReconnectWaitMs(10)).toBe(5000)
  })
})
