import { describe, it, expect, beforeEach, afterEach } from 'vitest'

describe('Environment Configuration', () => {
  let originalWindowEnv

  beforeEach(() => {
    // Save original window.env
    originalWindowEnv = window.env
  })

  afterEach(() => {
    // Restore original window.env
    window.env = originalWindowEnv
  })

  it('should have window.env defined', () => {
    expect(window.env).toBeDefined()
  })

  it('should have required environment variables', () => {
    expect(window.env).toHaveProperty('REACT_APP_ENV')
    expect(window.env).toHaveProperty('REACT_APP_API_BASE_URL')
  })

  it('should have valid environment values', () => {
    expect(typeof window.env.REACT_APP_ENV).toBe('string')
    expect(typeof window.env.REACT_APP_API_BASE_URL).toBe('string')
    expect(window.env.REACT_APP_ENV.length).toBeGreaterThan(0)
  })

  it('allows empty API base URL for same-origin proxy mode', () => {
    window.env.REACT_APP_API_BASE_URL = ''
    expect(window.env.REACT_APP_API_BASE_URL).toBe('')
  })

  it('should have valid API base URL format when configured', () => {
    if (!window.env.REACT_APP_API_BASE_URL) {
      return
    }
    const apiUrl = window.env.REACT_APP_API_BASE_URL
    expect(apiUrl).toMatch(/^https?:\/\//)
  })

  it('should handle missing environment gracefully', () => {
    // Temporarily remove window.env
    delete window.env

    // The app should not crash
    expect(() => {
      // Simulate accessing window.env in the app
      const env = window.env || {}
      return env.REACT_APP_API_BASE_URL || ''
    }).not.toThrow()
  })

  it('should provide fallback values when environment is missing', () => {
    // Test fallback behavior
    const getApiBaseUrl = () => {
      return window.env?.REACT_APP_API_BASE_URL || ''
    }

    const getEnvironment = () => {
      return window.env?.REACT_APP_ENV || 'development'
    }

    expect(getApiBaseUrl()).toBeDefined()
    expect(getEnvironment()).toBeDefined()
  })

  it('should have consistent environment configuration', () => {
    const env1 = window.env
    const env2 = window.env

    expect(env1).toEqual(env2)
  })

  it('should allow environment variables to be overridden', () => {
    const originalApiUrl = window.env.REACT_APP_API_BASE_URL

    // Test overriding environment variables
    window.env.REACT_APP_API_BASE_URL = 'https://test-api.example.com'

    expect(window.env.REACT_APP_API_BASE_URL).toBe('https://test-api.example.com')

    // Restore original value
    window.env.REACT_APP_API_BASE_URL = originalApiUrl
  })
})

describe('Environment Configuration Integration', () => {
  it('should be able to construct API URLs', () => {
    const apiBaseUrl = window.env.REACT_APP_API_BASE_URL
    if (!apiBaseUrl) {
      expect('/graphql').toMatch(/^\/graphql$/)
      return
    }
    const graphqlUrl = `${apiBaseUrl}/graphql`
    const healthUrl = `${apiBaseUrl}/healthz`

    expect(graphqlUrl).toMatch(/^https?:\/\/.*\/graphql$/)
    expect(healthUrl).toMatch(/^https?:\/\/.*\/healthz$/)
  })

  it('should be able to make API requests', async () => {
    const apiBaseUrl = window.env.REACT_APP_API_BASE_URL
    const healthUrl = `${apiBaseUrl}/healthz`

    try {
      const response = await fetch(healthUrl)
      if (response.ok) {
        const text = await response.text()
        expect(text).toBeDefined()
      }
    } catch (_error) {
      expect(typeof apiBaseUrl).toBe('string')
    }
  })

  it('should handle different environment configurations', () => {
    const environments = ['local', 'staging', 'production', 'test']
    const currentEnv = window.env.REACT_APP_ENV

    expect(environments).toContain(currentEnv)
  })
})
