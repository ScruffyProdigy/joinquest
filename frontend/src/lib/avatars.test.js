import { describe, it, expect } from 'vitest'
import {
  avatarInitial,
  defaultDisplayNameInput,
  resolveUserAvatarUrl,
  STARTER_AVATAR_FALLBACK,
} from './avatars'

describe('avatars', () => {
  it('builds initials from display name', () => {
    expect(avatarInitial({ displayName: 'Pat' })).toBe('P')
    expect(avatarInitial({})).toBe('P')
  })

  it('ships eighteen starter avatars', () => {
    expect(STARTER_AVATAR_FALLBACK).toHaveLength(18)
  })

  it('prefills the name, or nothing when there is none yet', () => {
    expect(defaultDisplayNameInput({ displayName: 'River' })).toBe('River')
    expect(defaultDisplayNameInput({ displayName: null })).toBe('')
    expect(defaultDisplayNameInput(null)).toBe('')
  })

  it('resolves avatar url from key when url is missing', () => {
    expect(resolveUserAvatarUrl({ avatarKey: 'storm' })).toBe('/avatars/storm.png')
    expect(resolveUserAvatarUrl({ avatarUrl: 'https://joinquest.cc/avatars/beacon.png' })).toBe(
      'https://joinquest.cc/avatars/beacon.png',
    )
  })
})
