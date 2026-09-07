import { describe, it, expect } from 'vitest'
import {
  chosenDisplayName,
  hasChosenAvatar,
  hasChosenDisplayName,
  identityGap,
  needsIdentity,
  viewerTier,
  IDENTITY_GAP_AVATAR,
  IDENTITY_GAP_BOTH,
  IDENTITY_GAP_NAME,
  VIEWER_GUEST,
  VIEWER_MEMBER,
  VIEWER_PASSIVE,
} from './viewer'

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
    // A nameless account is still a full account.
    expect(viewerTier({ isGuest: false, displayName: null })).toBe(VIEWER_MEMBER)
    // ...and a named guest is still a guest.
    expect(viewerTier({ isGuest: true, displayName: 'Ryan' })).toBe(VIEWER_GUEST)
  })
})

describe('hasChosenDisplayName', () => {
  it('is simply whether there is a name', () => {
    expect(hasChosenDisplayName({ displayName: 'Ryan' })).toBe(true)
    // A name that merely looks generated is still a name the player chose.
    expect(hasChosenDisplayName({ displayName: 'guest#135780Tester' })).toBe(true)
    expect(hasChosenDisplayName({ displayName: null })).toBe(false)
    expect(hasChosenDisplayName({ displayName: '   ' })).toBe(false)
    expect(hasChosenDisplayName({})).toBe(false)
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
    expect(chosenDisplayName({ displayName: 'Ryan' })).toBe('Ryan')
    expect(chosenDisplayName({ displayName: null })).toBe('')
    expect(chosenDisplayName(null)).toBe('')
  })
})

describe('needsIdentity', () => {
  const complete = { displayName: 'FrostFox4827', avatarKey: 'sigil-canine' }

  it('prompts a visitor with no session', () => {
    expect(needsIdentity(null)).toBe(true)
  })

  it('prompts a fresh guest who has neither', () => {
    expect(needsIdentity({ displayName: null, avatarKey: '' })).toBe(true)
  })

  it('prompts when only the avatar is missing', () => {
    expect(needsIdentity({ displayName: 'Ryan', avatarKey: '' })).toBe(true)
  })

  it('prompts when only the name is missing', () => {
    expect(needsIdentity({ displayName: null, avatarKey: 'sigil-canine' })).toBe(true)
  })

  it('leaves a guest who already picked both alone', () => {
    expect(needsIdentity({ ...complete, isGuest: true })).toBe(false)
  })

  it('leaves a signed-in player with a spirit animal alone', () => {
    expect(needsIdentity({ displayName: 'Ryan', avatarUrl: 'https://cdn/spirit.png' })).toBe(false)
  })
})

describe('identityGap', () => {
  it('reads a visitor with no session as missing both halves', () => {
    expect(identityGap(null)).toBe(IDENTITY_GAP_BOTH)
    expect(identityGap({ displayName: null, avatarKey: '', avatarUrl: '' })).toBe(IDENTITY_GAP_BOTH)
  })

  it('asks only for the name when a spirit animal is already on file', () => {
    // The state migration 000045 left every derived name in: an avatar, no name.
    expect(
      identityGap({ displayName: null, avatarKey: null, avatarUrl: '/avatars/fox.png', avatarSource: 'SPIRIT_ANIMAL' }),
    ).toBe(IDENTITY_GAP_NAME)
  })

  it('treats a blank name as no name', () => {
    expect(identityGap({ displayName: '   ', avatarKey: 'compass' })).toBe(IDENTITY_GAP_NAME)
  })

  it('asks only for the avatar when the name is chosen', () => {
    expect(identityGap({ displayName: 'Ryan', avatarKey: '', avatarUrl: '' })).toBe(IDENTITY_GAP_AVATAR)
  })

  it('is null once both halves are on file', () => {
    expect(identityGap({ displayName: 'Ryan', avatarKey: 'compass' })).toBeNull()
  })

  it('agrees with needsIdentity in every state', () => {
    const states = [
      null,
      { displayName: null, avatarKey: '' },
      { displayName: null, avatarSource: 'SPIRIT_ANIMAL' },
      { displayName: 'Ryan', avatarKey: '' },
      { displayName: 'Ryan', avatarKey: 'compass' },
    ]
    for (const user of states) {
      expect(needsIdentity(user)).toBe(identityGap(user) !== null)
    }
  })
})
