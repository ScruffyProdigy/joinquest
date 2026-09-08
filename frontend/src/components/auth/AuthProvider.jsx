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
  // Bumped every time the session is torn down. A request issued under an older
  // generation belongs to a session that no longer exists, so its result must be
  // dropped rather than written back — otherwise a `myAccount` or `me` response
  // still in flight when the user logs out re-seats the user they just cleared
  // (JQ-205). Callers capture this before awaiting and check it after.
  const sessionGenerationRef = useRef(0)

  const getSessionGeneration = useCallback(() => sessionGenerationRef.current, [])

  const setSessionUser = useCallback((nextUser) => {
    if (isSameSessionUser(userRef.current, nextUser)) {
      return
    }
    if (!nextUser) {
      // Signing out — whether through `clearSession` or a refresh that finds the
      // session gone — supersedes everything issued under the old session.
      sessionGenerationRef.current += 1
    }
    userRef.current = nextUser
    setUser(nextUser)
  }, [])

  const acceptSessionUser = useCallback(
    (signedInUser, options = {}) => {
      if (!signedInUser) {
        return
      }
      // Optional on purpose: the sign-in and profile call sites hand over a user
      // the player just produced by hand, so there is nothing stale to guard.
      // Anything fetched in the background passes the generation it read at
      // issue time and is dropped here if a logout landed in between.
      const { sessionGeneration } = options
      if (sessionGeneration !== undefined && sessionGeneration !== sessionGenerationRef.current) {
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
    // Retries stretch this call to ~7s, which is the widest window in the app for
    // a logout to land underneath it.
    const generation = sessionGenerationRef.current
    clearSubscriptionAuthCache()
    if (!silent) {
      setLoading(true)
    }
    setError('')
    setSessionUnavailable(false)
    try {
      const currentUser = await fetchCurrentUserWithRetries()
      if (generation !== sessionGenerationRef.current) {
        return
      }
      if (currentUser) {
        setSessionUser(currentUser)
        void prefetchSubscriptionAuth().catch(() => {})
      } else if (!silent) {
        setSessionUser(null)
      }
    } catch (err) {
      if (generation !== sessionGenerationRef.current) {
        return
      }
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
      getSessionGeneration,
    }),
    [
      user,
      loading,
      error,
      sessionUnavailable,
      refreshSession,
      acceptSessionUser,
      clearSession,
      getSessionGeneration,
    ],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// The hook lives next to the provider on purpose — it is useless without it.
// That costs this file fast refresh, which is a fair trade for one module.
// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}
