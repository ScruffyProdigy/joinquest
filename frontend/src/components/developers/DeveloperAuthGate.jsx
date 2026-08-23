import { Button } from '../ui/button'
import SignInPanel from '../auth/SignInPanel'
import { useAuth } from '../auth/AuthProvider'

const GATE_HEADING = 'Build on JoinQuest'
const GATE_SUBHEADING =
  'Create an account to register your game, connect via MCP, and manage your integration.'
const SIGNED_OUT_NOTICE =
  'A free account is required — guest play doesn’t support game registration or MCP.'
const GUEST_NOTICE =
  'You’re playing as a guest. Add an email to your profile to register a game — your handle and history come with you.'

/**
 * Soft gate for developer actions that need a durable identity.
 * Guests upgrade by linking an email; signed-out visitors sign up outright.
 */
export default function DeveloperAuthGate({ onBack, next = null }) {
  const { user } = useAuth()
  const isGuest = Boolean(user?.isGuest)

  return (
    <section className="developer-auth-gate" aria-labelledby="developer-auth-gate-heading">
      <header className="app-header">
        <h2 id="developer-auth-gate-heading">{GATE_HEADING}</h2>
        <p className="panel-copy">{GATE_SUBHEADING}</p>
      </header>

      <p className="status-message developer-auth-gate__notice">
        {isGuest ? GUEST_NOTICE : SIGNED_OUT_NOTICE}
      </p>

      {isGuest ? (
        <p className="developer-auth-gate__account">
          <Button variant="default" asChild>
            <a href="/account">Add an email to your account</a>
          </Button>
        </p>
      ) : (
        <SignInPanel heading={null} showGuestOption={false} next={next} />
      )}

      <p className="developer-route-back">
        <Button type="button" variant="link" onClick={onBack}>
          ← Back
        </Button>
      </p>
    </section>
  )
}
