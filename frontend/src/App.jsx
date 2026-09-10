import CompleteSignInPage from './components/auth/CompleteSignInPage'
import LinkEmailPage from './components/auth/LinkEmailPage'
import OAuthCompletePage from './components/auth/OAuthCompletePage'
import AccountPage from './components/auth/AccountPage'
import ReturnPage from './components/auth/ReturnPage'
import StylePreviewPage from './components/dev/StylePreviewPage'
import { isStylePreviewEnabled } from './lib/stylePreview'
import IntentBanner from './components/games/IntentBanner'
import GameLobby from './components/games/GameLobby'
import GameDetailPage from './components/games/GameDetailPage'
import WaitingPage from './components/games/WaitingPage'
import { ActiveRoomProvider } from './components/rooms/ActiveRoomProvider'
import { AuthProvider, useAuth } from './components/auth/AuthProvider'
import { useActiveIntent } from './components/games/useActiveIntent'
import { APP_NAME } from './lib/brand'
import { parseRoomInviteCode } from './lib/rooms'
import { parseGroupRoute } from './lib/group'
import GroupPage from './components/group/GroupPage'
import { parseGameSlug } from './lib/games'
import { navigateToWaiting, parseWaitingRoute } from './lib/waiting'
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

function CatalogPage({ intent }) {
  const { user, loading: authLoading } = useAuth()
  const { activeIntent, activeTableSeat, busy, leaveError, rejoinError, rejoining, handleLeave, handleRejoin } =
    intent

  useEffect(() => {
    restoreCatalogScrollIfPending()
  }, [])

  return (
    <main className="app-shell app-shell--catalog">
      {!authLoading && user ? (
        <IntentBanner
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          busy={busy}
          leaveError={leaveError}
          rejoinError={rejoinError}
          rejoining={rejoining}
          onLeave={handleLeave}
          onRejoin={handleRejoin}
        />
      ) : null}

      <HomeHeader />

      <GameLobby />
      <YourGamesStrip />

      <AppFooter />
    </main>
  )
}

function GameDetailShell({ slug, intent }) {
  const { user, loading: authLoading } = useAuth()
  const {
    activeIntent,
    activeTableSeat,
    busy,
    leaveError,
    rejoinError,
    rejoining,
    refresh,
    notifyQueueJoined,
    handleLeave,
    handleRejoin,
  } = intent

  // Joining a queue is the only route onto the waiting page.
  function handleQueueJoined(queueId, result, meta) {
    notifyQueueJoined(queueId, result, meta)
    if (result?.queued) {
      navigateToWaiting()
    }
  }

  return (
    <>
      {!authLoading && user ? (
        <IntentBanner
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          busy={busy}
          leaveError={leaveError}
          rejoinError={rejoinError}
          rejoining={rejoining}
          onLeave={handleLeave}
          onRejoin={handleRejoin}
        />
      ) : null}
      <GameDetailPage
        slug={slug}
        activeIntent={activeIntent}
        activeTableSeat={activeTableSeat}
        onQueueChange={refresh}
        onQueueJoined={handleQueueJoined}
        onTableChange={refresh}
      />
    </>
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
  // One instance for the whole shell. It has to survive the navigation from a game
  // page to /waiting, which carries the optimistic join state and its grace window.
  const intent = useActiveIntent()

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

  return onGroup ? (
    <GroupPage />
  ) : onWaiting ? (
    <WaitingPage intent={intent} />
  ) : gameSlug ? (
    <GameDetailShell slug={gameSlug} intent={intent} />
  ) : (
    <CatalogPage intent={intent} />
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
      </IdentityPromptProvider>
    </AuthProvider>
  )
}

export default App
