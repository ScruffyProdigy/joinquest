import { describe, it, expect, vi, afterEach } from 'vitest'
import { graphqlRequest, isAuthRequiredError, isIdentityRequiredError } from './graphql'
import { onIdentityRequired } from './identityPrompt'

function respondWith(payload, ok = true, status = 200) {
  return vi.fn().mockResolvedValue({
    ok,
    status,
    json: async () => payload,
    clone: () => ({ json: async () => payload }),
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('isIdentityRequiredError', () => {
  it('recognises the backend identity rejection', () => {
    expect(isIdentityRequiredError('identity required')).toBe(true)
    expect(isIdentityRequiredError('Identity Required')).toBe(true)
  })

  it('stays apart from a plain auth failure', () => {
    expect(isIdentityRequiredError('authentication required')).toBe(false)
    expect(isAuthRequiredError('identity required')).toBe(false)
    expect(isIdentityRequiredError('')).toBe(false)
    expect(isIdentityRequiredError(null)).toBe(false)
  })
})

describe('graphqlRequest', () => {
  it('raises the identity prompt when the backend rejects a nameless caller', async () => {
    vi.stubGlobal('fetch', respondWith({ errors: [{ message: 'identity required' }] }))
    const raised = vi.fn()
    const unsubscribe = onIdentityRequired(raised)

    await expect(graphqlRequest('mutation { createRoom { id } }')).rejects.toThrow(
      /identity required/i,
    )
    expect(raised).toHaveBeenCalledTimes(1)

    unsubscribe()
  })

  it('leaves other errors alone', async () => {
    vi.stubGlobal('fetch', respondWith({ errors: [{ message: 'authentication required' }] }))
    const raised = vi.fn()
    const unsubscribe = onIdentityRequired(raised)

    await expect(graphqlRequest('mutation { createRoom { id } }')).rejects.toThrow(
      /authentication required/i,
    )
    expect(raised).not.toHaveBeenCalled()

    unsubscribe()
  })

  it('stops notifying once unsubscribed', async () => {
    vi.stubGlobal('fetch', respondWith({ errors: [{ message: 'identity required' }] }))
    const raised = vi.fn()
    onIdentityRequired(raised)()

    await expect(graphqlRequest('mutation { createRoom { id } }')).rejects.toThrow()
    expect(raised).not.toHaveBeenCalled()
  })
})
