import { createContext, useContext } from 'react'
import { useActiveIntent } from './useActiveIntent'

const ActiveIntentContext = createContext(null)

/**
 * One `useActiveIntent` for the whole app (JQ-261).
 *
 * It used to be instantiated in `MainLayout`, which meant it did not exist on
 * `/account` -- so a player who lost their tab and came back through an account link
 * had no live-match surface at all. Hoisting it here is what lets the match dialog
 * mean "anywhere in the lobby" rather than "the pages the banner was mounted on".
 *
 * It has to stay a single instance: the hook carries the optimistic join state and
 * its grace window across the navigation from a game page to `/waiting`, and every
 * copy would open its own queue and table-seat subscriptions.
 */
export function ActiveIntentProvider({ children }) {
  const intent = useActiveIntent()
  return <ActiveIntentContext.Provider value={intent}>{children}</ActiveIntentContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useActiveIntentContext() {
  return useContext(ActiveIntentContext)
}
