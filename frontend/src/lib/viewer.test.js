import { describe, it, expect } from 'vitest'
import {
  chosenDisplayName,
  hasChosenAvatar,
  hasChosenDisplayName,
  needsIdentity,
  viewerTier,
  VIEWER_GUEST,
  VIEWER_MEMBER,
  VIEWER_PASSIVE,
} from './viewer'

const CHOSEN_AT = '2026-01-01T00:00:00Z'

describe('viewerTier', () => {
  it('reads no session as a passive viewer', () => {
    expect(viewerTier(null)).toBe(VIEWER_PASSIVE)
    expect(viewerTier(undefined)).toBe(VIEWER_PASSIVE)
  })

  it('separates guests from full accounts on isGuest alone', () => {
    expect(viewerTier({ isGuest: true })).toBe(VIEWER_GUEST)
    expect(viewerTier({ isGuest: false })).toBe(VIEWER_MEMBER)
  })

  it('does not consult the display name', () => {
    // A guest-looking name on a real account is still a full account.
    expect(viewerTier({ isGuest: false, displayName: 'guest#421900' })).toBe(VIEWER_MEMBER)
    // ...and a real-looking name on a guest is still a guest.
    expect(viewerTier({ isGuest: true, displayName: 'Ryan' })).toBe(VIEWER_GUEST)
  })
})

describe('hasChosenDisplayName', () => {
  it('follows the timestamp, not the string', () => {
    expect(hasChosenDisplayName({ displayName: 'Ryan', displayNameChosenAt: CHOSEN_AT })).toBe(true)
    expect(hasChosenDisplayName({ displayName: 'Ryan' })).toBe(false)
    // The shape that used to slip past the guest#NNNNNN regex.
    expect(hasChosenDisplayName({ displayName: 'guest#135780Tester' })).toBe(false)
    expect(
      hasChosenDisplayName({ displayName: 'guest#135780Tester', displayNameChosenAt: CHOSEN_AT }),
    ).toBe(true)
    expect(hasChosenDisplayName(null)).toBe(false)
  })
})

describe('hasChosenAvatar', () => {
  it('accepts a key, a url, or a spirit animal', () => {
    expect(hasChosenAvatar({ avatarKey: 'compass' })).toBe(true)
    expect(hasChosenAvatar({ avatarUrl: 'https://joinquest.cc/avatars/spirit/wolf.png' })).toBe(true)
    expect(hasChosenAvatar({ avatarSource: 'SPIRIT_ANIMAL' })).toBe(true)
    expect(hasChosenAvatar({ displayName: 'Pat' })).toBe(false)
    expect(hasChosenAvatar({ avatarKey: '   ' })).toBe(false)
  })
})

describe('chosenDisplayName', () => {
  it('gives back a name only once it was chosen', () => {
    expect(chosenDisplayName({ displayName: 'Ryan', displayNameChosenAt: CHOSEN_AT })).toBe('Ryan')
    expect(chosenDisplayName({ displayName: 'guest#421900' })).toBe('')
    expect(chosenDisplayName(null)).toBe('')
  })
})

describe('needsIdentity', () => {
  const complete = { displayName: 'FrostFox4827', displayNameChosenAt: CHOSEN_AT, avatarKey: 'sigil-canine' }

  it('prompts a visitor with no session', () => {
    expect(needsIdentity(null)).toBe(true)
  })

  it('prompts a fresh guest who has neither', () => {
    expect(needsIdentity({ displayName: 'guest#421900', avatarKey: '' })).toBe(true)
  })

  it('prompts when only the avatar is missing', () => {
    expect(needsIdentity({ displayName: 'Ryan', displayNameChosenAt: CHOSEN_AT, avatarKey: '' })).toBe(true)
  })

  it('prompts when only the name was never chosen', () => {
    expect(needsIdentity({ displayName: 'guest#421900', avatarKey: 'sigil-canine' })).toBe(true)
  })

  it('leaves a guest who already picked both alone', () => {
    expect(needsIdentity({ ...complete, isGuest: true })).toBe(false)
  })

  it('leaves a signed-in player with a spirit animal alone', () => {
    expect(
      needsIdentity({ displayName: 'Ryan', displayNameChosenAt: CHOSEN_AT, avatarUrl: 'https://cdn/spirit.png' }),
    ).toBe(false)
  })
})
