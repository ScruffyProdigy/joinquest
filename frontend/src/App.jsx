import CompleteSignInPage from './components/auth/CompleteSignInPage'
import LinkEmailPage from './components/auth/LinkEmailPage'
import OAuthCompletePage from './components/auth/OAuthCompletePage'
import AccountPage from './components/auth/AccountPage'
import ReturnPage from './components/auth/ReturnPage'
import StylePreviewPage from './components/dev/StylePreviewPage'
import { isStylePreviewEnabled } from './lib/stylePreview'
import GameLobby from './components/games/GameLobby'
import GameDetailPage from './components/games/GameDetailPage'
import WaitingPage from './components/games/WaitingPage'
import { ActiveRoomProvider } from './components/rooms/ActiveRoomProvider'
import { AuthProvider } from './components/auth/AuthProvider'
import ActiveMatchDialog from './components/games/ActiveMatchDialog'
import { ActiveIntentProvider, useActiveIntentContext } from './components/games/ActiveIntentProvider'
import { APP_NAME } from './lib/brand'
import { parseRoomInviteCode } from './lib/rooms'
import { parseGroupRoute } from './lib/group'
import GroupPage from './components/group/GroupPage'
import { parseGameSlug } from './lib/games'
import { navigateToWaiting, parseWaitingRoute } from './lib/waiting'
import { listenForPushMessages } from './lib/pushMessages'
import { usePathname } from './lib/usePathname'
import { restoreCatalogScrollIfPending } from './lib/catalogNavigation'
import { parseDeveloperRoute } from './lib/developers'
import DeveloperDashboard from './components/developers/DeveloperDashboard'
import DeveloperLandingPage from './components/developers/DeveloperLandingPage'
import DeveloperWelcomePage from './components/developers/DeveloperWelcomePage'
import YourGamesStrip from './components/developers/YourGamesStrip'
import HomeHeader from './components/home/HomeHeader'
import IdentityPromptProvider, { useIdentityPromptOnMount } from './components/avatars/IdentityPromptProvider'
import AppFooter from './components/legal/AppFooter'
import TermsPage from './components/legal/TermsPage'
import PrivacyPage from './components/legal/PrivacyPage'
import { useEffect } from 'react'
import { Link } from './components/ui/link'

function CatalogPage() {
  useEffect(() => {
    restoreCatalogScrollIfPending()
  }, [])

  return (
    <main className="app-shell app-shell--catalog">
      <HomeHeader />

      <GameLobby />
      <YourGamesStrip />

      <AppFooter />
    </main>
  )
}

function GameDetailShell({ slug, intent }) {
  const { activeIntent, activeTableSeat, refresh, notifyQueueJoined } = intent

  // Joining a queue is the only route onto the waiting page.
  function handleQueueJoined(queueId, result, meta) {
    notifyQueueJoined(queueId, result, meta)
    if (result?.queued) {
      navigateToWaiting()
    }
  }

  return (
    <GameDetailPage
        slug={slug}
        activeIntent={activeIntent}
        activeTableSeat={activeTableSeat}
        onQueueChange={refresh}
      onQueueJoined={handleQueueJoined}
      onTableChange={refresh}
    />
  )
}

// The dock, the room sheet, and the desktop room panel are gone (JQ-206): the
// Figma demo has no bottom nav and no room to dock to, and surfaces the demo
// does not have come out of production's UI. The room itself is untouched —
// ActiveRoomProvider still joins, subscribes and tracks membership, and the
// group screen is the presentation over it.
function MainLayout() {
  const pathname = usePathname()
  const gameSlug = parseGameSlug(pathname)
  const onGroup = parseGroupRoute(pathname)
  const onWaiting = parseWaitingRoute(pathname)
  const intent = useActiveIntentContext()

  useEffect(() => {
    const root = document.getElementById('root')
    root?.classList.toggle('app-root--game-detail', Boolean(gameSlug))
    // The group screen bleeds to the edges: its header is the top of the screen and
    // every block below carries its own gutter (JQ-251).
    root?.classList.toggle('app-root--group', onGroup)
    return () => {
      root?.classList.remove('app-root--game-detail')
      root?.classList.remove('app-root--group')
    }
  }, [gameSlug, onGroup])

  // The service worker cannot route or re-register on its own, so the shell
  // listens for the whole session rather than only while on /waiting -- a push
  // can land after the player has navigated away.
  useEffect(() => listenForPushMessages({
    // The worker navigates the tab itself where it can; this covers the case
    // where it could not.
    onNotificationClick: (url) => {
      if (url && url !== window.location.pathname) {
        window.location.assign(url)
      }
    },
  }), [])

  return onGroup ? (
    // The group screen needs the intent for the same reason the waiting page does:
    // a started table seat is how a player who did not press Start learns the game
    // began, and the launch moment reads it.
    <GroupPage intent={intent} />
  ) : onWaiting ? (
    <WaitingPage intent={intent} />
  ) : gameSlug ? (
    <GameDetailShell slug={gameSlug} intent={intent} />
  ) : (
    <CatalogPage />
  )
}

function DeveloperShell() {
  const pathname = usePathname()
  const route = parseDeveloperRoute(pathname)

  let page
  if (route?.kind === 'welcome') {
    page = <DeveloperWelcomePage gameId={route.gameId} />
  } else if (route?.kind === 'dashboard') {
    page = <DeveloperDashboard gameId={route.gameId} />
  } else {
    page = <DeveloperLandingPage />
  }

  return (
    <>
      {page}
      <AppFooter />
    </>
  )
}

function MainShell() {
  const pathname = usePathname()
  const inviteCode = parseRoomInviteCode(pathname)
  // Browsing the catalog and game pages stays open to a nameless visitor. An
  // invite link is different: following it is already an intent to join, so the
  // picker goes up before the visitor reaches the room. Every other surface
  // raises it from the action itself, through IdentityPromptProvider.
  useIdentityPromptOnMount(Boolean(inviteCode))

  return (
    <ActiveRoomProvider pendingInviteCode={inviteCode}>
      <MainLayout />
    </ActiveRoomProvider>
  )
}

/**
 * Where the live-match dialog is allowed to appear (JQ-261).
 *
 * Everywhere, with three deliberate holes.
 *
 * `/waiting` and `/group` already render `LaunchStep` themselves for a match that has
 * just formed -- that is an event, and this dialog is a recovery; the same component
 * in both at once would stack.
 *
 * `/return` is the post-match screen, and it is the one place where a live match and
 * a player who should not be in it are both correct at the same time. A session the
 * game has not reported a finish for is still `MATCHED` while the player stands on
 * that screen reading "Still playing", so a dialog here would carry them straight
 * back into the match they just walked out of, and do it again on every return.
 *
 * An `/auth/*` route is mid-sign-in, where interrupting costs more than the five
 * seconds the player would otherwise wait.
 */
function suppressesActiveMatchDialog(pathname) {
  return (
    parseWaitingRoute(pathname) ||
    parseGroupRoute(pathname) ||
    pathname.startsWith('/return') ||
    pathname.startsWith('/auth/')
  )
}

function ActiveMatchDialogHost() {
  const pathname = usePathname()
  const intent = useActiveIntentContext()

  if (!intent || suppressesActiveMatchDialog(pathname)) {
    return null
  }

  return (
    <ActiveMatchDialog
      activeIntent={intent.activeIntent}
      activeTableSeat={intent.activeTableSeat}
      busy={intent.busy}
      leaveError={intent.leaveError}
      rejoinError={intent.rejoinError}
      rejoining={intent.rejoining}
      onLeave={intent.handleLeave}
      onRejoin={intent.handleRejoin}
    />
  )
}

function App() {
  const pathname = usePathname()
  const developerRoute = parseDeveloperRoute(pathname)
  const isMainRoute =
    pathname === '/' ||
    parseGroupRoute(pathname) ||
    parseWaitingRoute(pathname) ||
    Boolean(parseRoomInviteCode(pathname)) ||
    Boolean(parseGameSlug(pathname))

  return (
    <AuthProvider>
      <ActiveIntentProvider>
        <IdentityPromptProvider>
          {pathname.startsWith('/auth/oauth/complete') ? (
            <OAuthCompletePage />
          ) : pathname.startsWith('/auth/complete') ? (
            <CompleteSignInPage />
          ) : pathname.startsWith('/auth/link') ? (
            <LinkEmailPage />
          ) : pathname.startsWith('/terms') ? (
            <TermsPage />
          ) : pathname.startsWith('/privacy') ? (
            <PrivacyPage />
          ) : pathname.startsWith('/account') ? (
            <AccountPage />
          ) : pathname.startsWith('/dev/style-preview') && isStylePreviewEnabled() ? (
            <StylePreviewPage />
          ) : pathname.startsWith('/return') ? (
            <ReturnPage />
          ) : developerRoute ? (
            <ActiveRoomProvider>
              <DeveloperShell />
            </ActiveRoomProvider>
          ) : isMainRoute ? (
            <MainShell />
          ) : (
            <main className="app-shell auth-page">
              <h1>Page not found</h1>
              <Link className="self-start" href="/">
                Back to {APP_NAME}
              </Link>
            </main>
          )}

          {/* Last child, and outside the route switch: a live match outranks
              whatever page the player thought they were opening (JQ-261). */}
          <ActiveMatchDialogHost />
        </IdentityPromptProvider>
      </ActiveIntentProvider>
    </AuthProvider>
  )
}

export default App
