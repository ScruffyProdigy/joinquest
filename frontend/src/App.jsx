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
import { ActiveRoomProvider, useActiveRoom } from './components/rooms/ActiveRoomProvider'
import AppDock from './components/rooms/AppDock'
import RoomPanel from './components/rooms/RoomPanel'
import RoomSheet from './components/rooms/RoomSheet'
import { AuthProvider, useAuth } from './components/auth/AuthProvider'
import { useActiveIntent } from './components/games/useActiveIntent'
import { APP_NAME } from './lib/brand'
import { onIdentityRequired } from './lib/identityPrompt'
import { parseRoomInviteCode } from './lib/rooms'
import { parseGameSlug } from './lib/games'
import { MOBILE_ROOM_QUERY, useMediaQuery } from './lib/useMediaQuery'
import { usePathname } from './lib/usePathname'
import { restoreCatalogScrollIfPending } from './lib/catalogNavigation'
import { parseDeveloperRoute } from './lib/developers'
import DeveloperDashboard from './components/developers/DeveloperDashboard'
import DeveloperLandingPage from './components/developers/DeveloperLandingPage'
import DeveloperWelcomePage from './components/developers/DeveloperWelcomePage'
import YourGamesStrip from './components/developers/YourGamesStrip'
import HomeHeader, { HOME_HEADING_ID } from './components/home/HomeHeader'
import IdentityGate from './components/avatars/IdentityGate'
import AppFooter from './components/legal/AppFooter'
import TermsPage from './components/legal/TermsPage'
import PrivacyPage from './components/legal/PrivacyPage'
import { useEffect, useState } from 'react'

function CatalogPage() {
  const { user, loading: authLoading } = useAuth()
  const { activeIntent, activeTableSeat, busy, queueWsConnected, handleLeave } =
    useActiveIntent()
  const liveUpdatesConnected = !activeIntent?.queueId || activeIntent.status !== 'WAITING' || queueWsConnected

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
          liveUpdatesConnected={liveUpdatesConnected}
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
  const { activeIntent, activeTableSeat, busy, queueWsConnected, refresh, notifyQueueJoined, handleLeave } =
    useActiveIntent()
  const liveUpdatesConnected = !activeIntent?.queueId || activeIntent.status !== 'WAITING' || queueWsConnected

  return (
    <>
      {!authLoading && user ? (
        <IntentBanner
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          busy={busy}
          liveUpdatesConnected={liveUpdatesConnected}
          onLeave={handleLeave}
        />
      ) : null}
      <GameDetailPage
        slug={slug}
        activeIntent={activeIntent}
        activeTableSeat={activeTableSeat}
        onQueueChange={refresh}
        onQueueJoined={notifyQueueJoined}
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
  const inRoomContext = Boolean(room || inviteCode || hasRoomMembership)
  // Desktop keeps the room panel visible whenever the user belongs to a room; mobile toggles via dock/sheet.
  const showDesktopRoom = Boolean(room && user && !isMobile)
  const showMobileSheet = Boolean(user && isMobile && roomOpen)
  const showDock = Boolean(user && isMobile && inRoomContext)

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
          {gameSlug ? <GameDetailShell slug={gameSlug} /> : <CatalogPage />}
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
  // The backend turns away anyone without a name and an avatar at every queue,
  // room and seat. That rejection is a cue to finish the identity prompt, not an
  // error to print, so raise the same picker here. IdentityGate hides itself
  // once both are chosen.
  const [identityRequired, setIdentityRequired] = useState(false)
  useEffect(() => onIdentityRequired(() => setIdentityRequired(true)), [])

  return (
    <ActiveRoomProvider pendingInviteCode={inviteCode}>
      <MainLayout />
      {/* Browsing the catalog and game pages stays open to guests, so the picker
          only blocks a room invite, where the visitor is joining a table. */}
      {inviteCode || identityRequired ? <IdentityGate /> : null}
    </ActiveRoomProvider>
  )
}

function App() {
  const pathname = usePathname()
  const developerRoute = parseDeveloperRoute(pathname)
  const isMainRoute =
    pathname === '/' ||
    Boolean(parseRoomInviteCode(pathname)) ||
    Boolean(parseGameSlug(pathname))

  return (
    <AuthProvider>
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
    </AuthProvider>
  )
}

export default App
