import { useRef, useState } from 'react'
import { isSoloMode, joinGroupOptionsForMode, modePlayerRangeLabel } from '../../lib/games'
import { accentColorFor } from '../../lib/gameAccent'
import { hasPlayingIntent, hasWaitingIntent } from '../../lib/intent'
import { PLAY_WITH_FRIENDS } from '../../lib/playerCopy'
import { createPrivateTable } from '../../lib/tables'
import { useIdentityPrompt } from '../avatars/IdentityPromptProvider'
import { useActiveRoom } from '../rooms/ActiveRoomProvider'
import GameQueueActions from './GameQueueActions'
import ModeRequirement from './ModeRequirement'
import PreQueueOptionsSheet from './PreQueueOptionsSheet'
import { useGameQueue } from './useGameQueue'
import { navigateTo } from '../../lib/usePathname'

function ModeRow({
  game,
  mode,
  activeIntent,
  activeTableSeat,
  onQueueChange,
  onQueueJoined,
  onTableChange,
  onNavigateToMode,
  prominent = false,
}) {
  const { refresh: refreshRoom } = useActiveRoom()
  const { requireIdentity } = useIdentityPrompt()
  const defaultQueue = mode.queues?.find((q) => q.status === 'active') ?? null
  const playerRangeLabel = modePlayerRangeLabel(mode)
  const accent = prominent ? accentColorFor(game.slug, game.accentColor) : null
  const joinOptions = joinGroupOptionsForMode(mode)
  const solo = isSoloMode(mode)
  const [tableBusy, setTableBusy] = useState(false)
  const [tableError, setTableError] = useState('')
  // A mode with option groups routes every join through the picker first, so
  // the pending path is held until the player has answered it.
  const preQueueGroups = mode.preQueueGroups ?? []
  const [pendingQueuePath, setPendingQueuePath] = useState(null)

  const isThisQueue =
    defaultQueue?.id && activeIntent?.queueId && activeIntent.queueId === defaultQueue.id
  const queue = useGameQueue(defaultQueue?.id, {
    skipSubscription: Boolean(isThisQueue && activeIntent),
  })

  const seatedHere =
    activeTableSeat?.gameId === game.id && activeTableSeat?.modeId === mode.id
  const inActiveGame = activeIntent?.status === 'MATCHED'

  async function handleJoin(queuePath, options) {
    if (inActiveGame) {
      setTableError('Launch or finish your current game before looking for a group.')
      return
    }
    if (activeTableSeat?.tableId) {
      setTableError('Leave your table seat before looking for a group.')
      return
    }
    const pathLabel =
      joinOptions?.kind === 'composition'
        ? joinOptions.paths
            .map((path) => (typeof path === 'string' ? { queuePath: path, displayName: path } : path))
            .find((entry) => entry.queuePath === queuePath)?.displayName
        : null
    // The queue, the path, and the options the player picked are the whole
    // intent, and they are captured in this closure — so if the identity
    // prompt goes up first, the same join runs by itself, picks included, once
    // a name and avatar are saved.
    await requireIdentity(async () => {
      const result = await queue.handleJoin(queuePath, options)
      if (result) {
        onQueueJoined?.(defaultQueue?.id, result, {
          gameId: game.id,
          gameName: game.name,
          modeName: mode.displayName,
          queuePathDisplayName: pathLabel ?? null,
        })
      }
    })
  }

  // The mode is already settled by the time this runs, and for a composition
  // mode so is the role — which keeps the order the prototype fixed:
  // game -> mode -> role -> options -> queue.
  async function handleJoinRequest(queuePath) {
    if (preQueueGroups.length === 0) {
      await handleJoin(queuePath)
      return
    }
    setTableError('')
    setPendingQueuePath(typeof queuePath === 'string' ? queuePath : '')
  }

  async function handleOptionsConfirmed(selections) {
    const queuePath = pendingQueuePath
    setPendingQueuePath(null)
    await handleJoin(queuePath, selections)
  }

  async function handleLeave() {
    await queue.handleLeave()
    await onQueueChange?.()
  }

  // requireIdentity replays the closure it was handed, and by then the session
  // is not the one the visitor clicked with. refreshRoom is safe on its own now
  // — ActiveRoomProvider reads the session at call time (JQ-202) — but
  // onTableChange is useActiveIntent's refresh, and that one is still rebuilt
  // per session and clears its state when it holds no user, so a replay of the
  // *captured* one wipes the banner for the table just created. Going through a
  // ref means the replay runs against the session the visitor now has.
  const runStartGroupRef = useRef(null)
  runStartGroupRef.current = async () => {
    await createPrivateTable(game.id, mode.id)
    await refreshRoom()
    await onTableChange?.()
    // The room is implicit: the player asked to play with friends, not to make a
    // room, so they land straight on their group (JQ-132).
    navigateTo('/group')
  }

  async function startGroup() {
    setTableBusy(true)
    setTableError('')
    try {
      // A visitor with no name and avatar is asked for one first, and this whole
      // sequence then runs by itself — so one click on "play with friends" still
      // ends in the group rather than back on the catalog (JQ-131).
      await requireIdentity(() => runStartGroupRef.current())
    } catch (err) {
      setTableError(err.message || 'Could not create private game.')
    } finally {
      setTableBusy(false)
    }
  }

  async function handleCreatePrivate() {
    if (inActiveGame) {
      setTableError('Launch or finish your current game before creating a private game.')
      return
    }
    if (activeTableSeat?.tableId && !seatedHere) {
      setTableError('Leave your current table seat first.')
      return
    }
    if (activeIntent?.queueId) {
      setTableError('Stop looking for a group before creating a private game.')
      return
    }
    await startGroup()
  }

  const blockedByMatch =
    inActiveGame ||
    (activeIntent &&
      defaultQueue?.id &&
      !isThisQueue &&
      queue.queueState === 'idle' &&
      activeIntent.status === 'MATCHED')

  const resolvedQueueState = isThisQueue
    ? hasWaitingIntent(activeIntent)
      ? 'waiting'
      : hasPlayingIntent(activeIntent)
        ? 'matched'
        : queue.queueState
    : queue.queueState
  const resolvedJoinUrl =
    isThisQueue && activeIntent?.joinUrl ? activeIntent.joinUrl : queue.joinUrl

  return (
    <li
      id={`game-mode-row-${game.id}-${mode.modeKey}`}
      className={`game-mode-row${prominent ? ' game-mode-row--prominent rounded-2xl overflow-hidden border-[1.5px] p-4 transition-[transform,box-shadow] duration-150 motion-reduce:transition-none hover:-translate-y-0.5 hover:shadow-lg hover:ring-1 hover:ring-white/15' : ''}`}
      style={prominent ? { background: accent.cardBg, borderColor: accent.border } : undefined}
    >
      <div className="game-mode-row__copy">
        <h4 className="game-mode-row__title">{mode.displayName}</h4>
        {prominent && playerRangeLabel ? (
          <ul className="mb-1 flex flex-wrap gap-1" aria-label="Mode details">
            <li
              className="rounded-full px-2 py-1 text-[11px] font-semibold backdrop-blur-[6px]"
              style={{ backgroundColor: 'rgba(0,0,0,0.42)', color: 'rgba(255,255,255,0.92)' }}
            >
              {playerRangeLabel}
            </li>
          </ul>
        ) : null}
        {tableError ? (
          <p className="status-message status-message-error" role="status">
            {tableError}
          </p>
        ) : null}
        {queue.error ? (
          <p className="status-message status-message-error" role="status">
            {queue.error}
          </p>
        ) : null}
        {queue.notice ? (
          <p className="status-message" role="status">
            {queue.notice}
          </p>
        ) : null}
        {seatedHere ? (
          <p className="game-list-meta" role="status">
            You are seated at a private table for this mode.
          </p>
        ) : null}
      </div>
      <div className="game-mode-row__actions">
        {mode.eligibility?.accessible === false ? (
          <div className="game-mode-row__locked" role="status">
            <button
              type="button"
              className="game-mode-row__locked-reason"
              disabled={!mode.eligibility.unlockModeKey}
              onClick={() => {
                if (mode.eligibility.unlockModeKey) {
                  onNavigateToMode?.(mode.eligibility.unlockModeKey)
                }
              }}
            >
              {mode.eligibility.reason}
            </button>
            {mode.eligibility.requirement ? (
              <p className="game-mode-row__locked-progress">
                <ModeRequirement requirement={mode.eligibility.requirement} />
              </p>
            ) : null}
          </div>
        ) : (
          <>
            <GameQueueActions
              joinOptions={joinOptions}
              queueState={resolvedQueueState}
              joinUrl={resolvedJoinUrl}
              busy={queue.busy}
              selectedQueuePath={
                queue.selectedQueuePath || (isThisQueue ? activeIntent?.queuePath : '') || ''
              }
              onJoin={handleJoinRequest}
              onLeave={handleLeave}
              disabled={!defaultQueue || blockedByMatch || Boolean(activeTableSeat?.tableId && !seatedHere)}
              prominent={prominent}
              solo={solo}
            />
            {solo ? null : (
              <button
                type="button"
                className={`game-list-button game-list-button-secondary${prominent ? ' game-list-button--prominent' : ''}`}
                disabled={tableBusy || blockedByMatch}
                onClick={handleCreatePrivate}
              >
                {tableBusy ? '…' : PLAY_WITH_FRIENDS}
              </button>
            )}
            <PreQueueOptionsSheet
              open={pendingQueuePath !== null}
              gameName={game.name}
              modeName={mode.displayName}
              groups={preQueueGroups}
              queueOptions={mode.queueOptions}
              busy={queue.busy}
              error={queue.error}
              onConfirm={handleOptionsConfirmed}
              onClose={() => setPendingQueuePath(null)}
            />
          </>
        )}
      </div>
    </li>
  )
}

export default function GameModesPanel({
  game,
  activeIntent,
  activeTableSeat,
  onQueueChange,
  onQueueJoined,
  onTableChange,
  onNavigateToMode,
  heading = 'Play',
  variant = 'default',
}) {
  const modes = (game?.modes ?? []).filter((mode) => mode.status === 'active')
  const prominent = variant === 'prominent'

  if (modes.length === 0) {
    return <p className="game-list-meta">No active modes right now.</p>
  }

  // When the caller doesn't wire up cross-page navigation, scroll the
  // unlocking mode's own row into view within this panel instead of no-oping.
  // The id is scoped by game.id so that panels for different games (each
  // rendered once per game on the lobby listing) never collide when two
  // games happen to share a mode key.
  const handleNavigateToMode =
    onNavigateToMode ??
    ((modeKey) => {
      document
        .getElementById(`game-mode-row-${game.id}-${modeKey}`)
        ?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    })

  return (
    <section className={`game-modes-panel${prominent ? ' game-modes-panel--prominent' : ''}`}>
      {heading ? <h3 className="game-modes-panel__title">{heading}</h3> : null}
      <ul className="game-mode-list">
        {modes.map((mode) => (
          <ModeRow
            key={mode.id ?? mode.modeKey}
            game={game}
            mode={mode}
            activeIntent={activeIntent}
            activeTableSeat={activeTableSeat}
            onQueueChange={onQueueChange}
            onQueueJoined={onQueueJoined}
            onTableChange={onTableChange}
            onNavigateToMode={handleNavigateToMode}
            prominent={prominent}
          />
        ))}
      </ul>
    </section>
  )
}
