import { homeGreetingLine } from '../../lib/greeting'
import { FIND_A_GAME_HEADING } from '../../lib/playerCopy'
import { chosenDisplayName } from '../../lib/viewer'
import { useAuth } from '../auth/AuthProvider'
import AccountChip from './AccountChip'

/** The id the catalog grid points at, so the two stay in sync. */
export const HOME_HEADING_ID = 'find-a-game-heading'

/**
 * Home opens on the catalog, so this header is deliberately small: who you are
 * on the left, the one auth control on the right, and the catalog's heading.
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
      <h1 id={HOME_HEADING_ID} className="home-header__heading">
        {FIND_A_GAME_HEADING}
      </h1>
    </header>
  )
}
