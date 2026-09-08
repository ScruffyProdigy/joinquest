import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { fetchCurrentUser } from '../../lib/auth'
import { isTransientServerError } from '../../lib/graphql'
import { clearSubscriptionAuthCache, prefetchSubscriptionAuth } from '../../lib/queue'

const SESSION_RETRY_ATTEMPTS = 4
const SESSION_RETRY_DELAY_MS = 750

async function fetchCurrentUserWithRetries() {
  let lastErr
  for (let attempt = 0; attempt < SESSION_RETRY_ATTEMPTS; attempt += 1) {
    try {
      return await fetchCurrentUser()
    } catch (err) {
      lastErr = err
      if (!isTransientServerError(err.message) || attempt === SESSION_RETRY_ATTEMPTS - 1) {
        throw err
      }
      await new Promise((resolve) => setTimeout(resolve, SESSION_RETRY_DELAY_MS * (attempt + 1)))
    }
  }
  throw lastErr
}

// A re-fetch that returns the same account must not count as a new session.
// The session user is a flat scalar record, so a shallow compare settles it;
// anything nested would compare unequal and simply fall through to an update.
function isSameSessionUser(a, b) {
  if (a === b) {
    return true
  }
  if (!a || !b) {
    return false
  }
  const keys = new Set([...Object.keys(a), ...Object.keys(b)])
  for (const key of keys) {
    if (!Object.is(a[key], b[key])) {
      return false
    }
  }
  return true
}

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [sessionUnavailable, setSessionUnavailable] = useState(false)
  // Mirrors `user` so the callbacks below can compare against it without taking
  // it as a dependency — a callback whose identity changed with the user would
  // re-trigger every effect keyed on it, which is the loop we are closing.
  const userRef = useRef(null)

  const setSessionUser = useCallback((nextUser) => {
    if (isSameSessionUser(userRef.current, nextUser)) {
      return
    }
    userRef.current = nextUser
    setUser(nextUser)
  }, [])

  const acceptSessionUser = useCallback(
    (signedInUser) => {
      if (!signedInUser) {
        return
      }
      // Same account, freshly parsed: re-seating it would drop the subscription
      // socket and hand every `user`-keyed effect a new object to react to.
      if (isSameSessionUser(userRef.current, signedInUser)) {
        return
      }
      clearSubscriptionAuthCache()
      setSessionUser(signedInUser)
      setError('')
      void prefetchSubscriptionAuth().catch(() => {})
    },
    [setSessionUser],
  )

  const refreshSession = useCallback(async (options = {}) => {
    const { silent = false } = options
    clearSubscriptionAuthCache()
    if (!silent) {
      setLoading(true)
    }
    setError('')
    setSessionUnavailable(false)
    try {
      const currentUser = await fetchCurrentUserWithRetries()
      if (currentUser) {
        setSessionUser(currentUser)
        void prefetchSubscriptionAuth().catch(() => {})
      } else if (!silent) {
        setSessionUser(null)
      }
    } catch (err) {
      if (!silent) {
        const message = err.message || 'Could not load session'
        if (isTransientServerError(message)) {
          setSessionUnavailable(true)
          setError('Server briefly unavailable — your session may still be active. Try again in a moment.')
        } else {
          setError(message)
          setSessionUser(null)
        }
      }
    } finally {
      if (!silent) {
        setLoading(false)
      }
    }
  }, [setSessionUser])

  const clearSession = useCallback(() => {
    clearSubscriptionAuthCache()
    setSessionUser(null)
    setError('')
    setSessionUnavailable(false)
  }, [setSessionUser])

  useEffect(() => {
    refreshSession()
  }, [refreshSession])

  const value = useMemo(
    () => ({
      user,
      loading,
      error,
      sessionUnavailable,
      refreshSession,
      acceptSessionUser,
      clearSession,
    }),
    [user, loading, error, sessionUnavailable, refreshSession, acceptSessionUser, clearSession],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}
