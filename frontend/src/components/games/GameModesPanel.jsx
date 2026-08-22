import { useState } from 'react'
import { joinGroupOptionsForMode } from '../../lib/games'
import { hasPlayingIntent, hasWaitingIntent } from '../../lib/intent'
import { CREATE_PRIVATE_GAME } from '../../lib/playerCopy'
import { createPrivateTable } from '../../lib/tables'
import { useActiveRoom } from '../rooms/ActiveRoomProvider'
import GameQueueActions from './GameQueueActions'
import ModeRequirement from './ModeRequirement'
import { useGameQueue } from './useGameQueue'

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
  const { refresh: refreshRoom, openRoom } = useActiveRoom()
  const defaultQueue = mode.queues?.find((q) => q.status === 'active') ?? null
  const joinOptions = joinGroupOptionsForMode(mode)
  const [tableBusy, setTableBusy] = useState(false)
  const [tableError, setTableError] = useState('')

  const isThisQueue =
    defaultQueue?.id && activeIntent?.queueId && activeIntent.queueId === defaultQueue.id
  const queue = useGameQueue(defaultQueue?.id, {
    skipSubscription: Boolean(isThisQueue && activeIntent),
  })

  const seatedHere =
    activeTableSeat?.gameId === game.id && activeTableSeat?.modeId === mode.id
  const inActiveGame = activeIntent?.status === 'MATCHED'

  async function handleJoin(queuePath) {
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
    const result = await queue.handleJoin(queuePath)
    if (result) {
      onQueueJoined?.(defaultQueue?.id, result, {
        gameId: game.id,
        gameName: game.name,
        modeName: mode.displayName,
        queuePathDisplayName: pathLabel ?? null,
      })
    }
  }

  async function handleLeave() {
    await queue.handleLeave()
    await onQueueChange?.()
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
    setTableBusy(true)
    setTableError('')
    try {
      await createPrivateTable(game.id, mode.id)
      await refreshRoom()
      openRoom()
      await onTableChange?.()
    } catch (err) {
      setTableError(err.message || 'Could not create private game.')
    } finally {
      setTableBusy(false)
    }
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
      className={`game-mode-row${prominent ? ' game-mode-row--prominent' : ''}`}
    >
      <div className="game-mode-row__copy">
        <h4 className="game-mode-row__title">{mode.displayName}</h4>
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
              onJoin={handleJoin}
              onLeave={handleLeave}
              disabled={!defaultQueue || blockedByMatch || Boolean(activeTableSeat?.tableId && !seatedHere)}
              prominent={prominent}
            />
            <button
              type="button"
              className={`game-list-button game-list-button-secondary${prominent ? ' game-list-button--prominent' : ''}`}
              disabled={tableBusy || blockedByMatch}
              onClick={handleCreatePrivate}
            >
              {tableBusy ? '…' : CREATE_PRIVATE_GAME}
            </button>
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
