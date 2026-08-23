import SignInPanel from './SignInPanel'
import UserSessionCard from './UserSessionCard'
import { useAuth } from './AuthProvider'

function SessionLoading() {
  return (
    <p className="status-message" role="status">
      Loading session…
    </p>
  )
}

export default function AuthPanel({ variant = 'default' }) {
  const { user, loading, error, sessionUnavailable, refreshSession } = useAuth()
  const isDeveloper = variant === 'developer'

  if (loading) {
    return <SessionLoading />
  }

  if (user) {
    return <UserSessionCard user={user} compact={isDeveloper} showProfileActions={!isDeveloper} />
  }

  const loginPanel = (
    <>
      {error ? <p className="status-message status-message-error">{error}</p> : null}
      {sessionUnavailable ? (
        <button type="button" className="button-secondary" onClick={() => void refreshSession()}>
          Try again
        </button>
      ) : (
        <SignInPanel />
      )}
    </>
  )

  if (isDeveloper) {
    return <section className="panel-card auth-panel auth-panel--developer">{loginPanel}</section>
  }

  return loginPanel
}
