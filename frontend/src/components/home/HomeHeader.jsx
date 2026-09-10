import { homeGreetingLine } from '../../lib/greeting'
import { APP_TAGLINE } from '../../lib/playerCopy'
import { chosenDisplayName } from '../../lib/viewer'
import { useAuth } from '../auth/AuthProvider'
import AccountChip from './AccountChip'

/**
 * Home opens on the catalog, so this header is deliberately small: who you are
 * on the left, the one auth control on the right, and the tagline.
 *
 * The tagline is the h1 because that is what the prototype leads with (JQ-249).
 * The catalog names itself with its own heading, in `GameLobby`, so the section
 * label sits with the section rather than being borrowed from up here.
 */
export default function HomeHeader() {
  const { user, loading } = useAuth()
  // Empty until the player picks a name, so we never greet someone by a
  // placeholder the backend handed out.
  const name = loading ? '' : chosenDisplayName(user)

  return (
    <header className="home-header">
      <div className="home-header__row">
        <p className="home-header__greeting">{homeGreetingLine(name)}</p>
        <AccountChip />
      </div>
      <h1 className="home-header__heading">{APP_TAGLINE}</h1>
    </header>
  )
}
