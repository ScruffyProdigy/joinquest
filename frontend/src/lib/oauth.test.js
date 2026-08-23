import { describe, expect, it } from 'vitest'
import { buildOAuthStartUrl } from './oauth'

describe('buildOAuthStartUrl', () => {
  it('builds a plain sign-in start URL', () => {
    expect(buildOAuthStartUrl('google')).toBe('/auth/oauth/google/start')
  })

  it('keeps link mode and merge confirmation', () => {
    const url = buildOAuthStartUrl('google', { mode: 'link', confirmMerge: true })

    expect(url).toContain('mode=link')
    expect(url).toContain('confirm_merge=1')
  })

  it('passes a next destination key through', () => {
    expect(buildOAuthStartUrl('google', { next: 'dev-manual' })).toBe(
      '/auth/oauth/google/start?next=dev-manual',
    )
  })

  it('omits next when it is not provided', () => {
    expect(buildOAuthStartUrl('google', { next: null })).toBe('/auth/oauth/google/start')
    expect(buildOAuthStartUrl('google', { next: '' })).toBe('/auth/oauth/google/start')
  })

  it('encodes a next value rather than letting it break out of the query', () => {
    // The backend is the security boundary, but the client must not build a malformed URL.
    const url = buildOAuthStartUrl('google', { next: 'a&b=c' })

    expect(url).toBe('/auth/oauth/google/start?next=a%26b%3Dc')
  })
})
