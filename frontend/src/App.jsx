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
import { ActiveRoomProvider, useActiveRoom } from './components/rooms/ActiveRoomProvider'
import AppDock from './components/rooms/AppDock'
import RoomPanel from './components/rooms/RoomPanel'
import RoomSheet from './components/rooms/RoomSheet'
import { AuthProvider, useAuth } from './components/auth/AuthProvider'
import { useActiveIntent } from './components/games/useActiveIntent'
import { APP_NAME } from './lib/brand'
import { parseRoomInviteCode } from './lib/rooms'
import { parseGroupRoute } from './lib/group'
import GroupPage from './components/group/GroupPage'
import { parseGameSlug } from './lib/games'
import { navigateToWaiting, parseWaitingRoute } from './lib/waiting'
import { MOBILE_ROOM_QUERY, useMediaQuery } from './lib/useMediaQuery'
import { usePathname } from './lib/usePathname'
import { restoreCatalogScrollIfPending } from './lib/catalogNavigation'
import { parseDeveloperRoute } from './lib/developers'
import DeveloperDashboard from './components/developers/DeveloperDashboard'
import DeveloperLandingPage from './components/developers/DeveloperLandingPage'
import DeveloperWelcomePage from './components/developers/DeveloperWelcomePage'
import YourGamesStrip from './components/developers/YourGamesStrip'
import HomeHeader, { HOME_HEADING_ID } from './components/home/HomeHeader'
import IdentityPromptProvider, { useIdentityPromptOnMount } from './components/avatars/IdentityPromptProvider'
import AppFooter from './components/legal/AppFooter'
import TermsPage from './components/legal/TermsPage'
import PrivacyPage from './components/legal/PrivacyPage'
import { useEffect } from 'react'

function CatalogPage() {
  const { user, loading: authLoading } = useAuth()
  const { activeIntent, activeTableSeat, busy, leaveError, handleLeave } = useActiveIntent()

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
          onLeave={handleLeave}
        />
      ) : null}

      <HomeHeader />

      <GameLobby headingId={HOME_HEADING_ID} />
      <YourGamesStrip />

      <AppFooter />
    </main>
  )
}

function GameDetailShell({ slug }) {
  const { user, loading: authLoading } = useAuth()
  const { activeIntent, activeTableSeat, busy, leaveError, refresh, notifyQueueJoined, handleLeave } =
    useActiveIntent()

  // Joining a queue is the only way onto the waiting page. Leaving it — Back, a link,
  // a closed tab — gives up the queue, in useLeaveQueueOnExit.
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
          onLeave={handleLeave}
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

function MainLayout() {
  const pathname = usePathname()
  const inviteCode = parseRoomInviteCode(pathname)
  const gameSlug = parseGameSlug(pathname)
  const { user } = useAuth()
  const { room, roomOpen, dismissRoom, openRoom, unreadCount, hasRoomMembership } = useActiveRoom()
  const isMobile = useMediaQuery(MOBILE_ROOM_QUERY)
  const onGroup = parseGroupRoute(pathname)
  const onWaiting = parseWaitingRoute(pathname)
  const inRoomContext = Boolean(room || inviteCode || hasRoomMembership)
  // Desktop keeps the room panel visible whenever the user belongs to a room; mobile toggles via dock/sheet.
  // The group page is the exception: it is a presentation over the same room, so the
  // room must not also show through beside it (JQ-132).
  const showDesktopRoom = Boolean(room && user && !isMobile && !onGroup)
  const showMobileSheet = Boolean(user && isMobile && roomOpen && !onGroup)
  const showDock = Boolean(user && isMobile && inRoomContext && !onGroup)

  useEffect(() => {
    const root = document.getElementById('root')
    root?.classList.toggle('app-root--split', showDesktopRoom)
    root?.classList.toggle('app-root--dock', showDock)
    root?.classList.toggle('app-root--game-detail', Boolean(gameSlug))
    return () => {
      root?.classList.remove('app-root--split', 'app-root--dock', 'app-root--game-detail')
    }
  }, [showDesktopRoom, showDock, gameSlug])

  return (
    <>
      <div className={`app-layout ${showDesktopRoom ? 'app-layout--split' : ''}`}>
        <div className="app-layout__catalog">
          {onGroup ? (
            <GroupPage />
          ) : onWaiting ? (
            <WaitingPage />
          ) : gameSlug ? (
            <GameDetailShell slug={gameSlug} />
          ) : (
            <CatalogPage />
          )}
        </div>
        {showDesktopRoom ? (
          <aside className="app-layout__room" aria-label="Room chat">
            <RoomPanel compact />
          </aside>
        ) : null}
      </div>

      {showMobileSheet ? (
        <RoomSheet open={roomOpen} onDismiss={dismissRoom}>
          <RoomPanel compact />
        </RoomSheet>
      ) : null}

      {showDock ? (
        <AppDock
          unreadCount={unreadCount}
          roomOpen={roomOpen}
          onOpenCatalog={dismissRoom}
          onOpenRoom={openRoom}
        />
      ) : null}
    </>
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
            <a className="auth-link" href="/">
              Back to {APP_NAME}
            </a>
          </main>
        )}
      </IdentityPromptProvider>
    </AuthProvider>
  )
}

export default App
