import {
  LAUNCH_GAME,
  LEAVE_MATCH,
  LOOKING_FOR_GROUP,
  LOOK_FOR_GROUP,
  PLAY_SOLO,
  STARTING_SOLO,
  STOP_LOOKING,
  joinAsLabel,
  waitingAsRoleLine,
} from '../../lib/playerCopy'
import { Button } from '../ui/button'

function pathOption(path) {
  if (typeof path === 'string') {
    return { queuePath: path, displayName: path }
  }
  return {
    queuePath: path.queuePath,
    displayName: path.displayName || path.queuePath,
  }
}

function JoinPathButton({ path, busy, disabled, onJoin, prominent = false }) {
  const { queuePath, displayName } = pathOption(path)
  return (
    <Button
      size={prominent ? 'lg' : 'default'}
      className={prominent ? 'w-full' : undefined}
      onClick={() => onJoin(queuePath)}
      disabled={busy || disabled}
    >
      {busy ? LOOKING_FOR_GROUP : joinAsLabel(displayName)}
    </Button>
  )
}

function JoinGroupPanel({ children, prominent = false }) {
  return (
    <section
      className={`join-group-panel${prominent ? ' join-group-panel--prominent' : ''}`}
      aria-label={LOOK_FOR_GROUP}
    >
      <p className="join-group-panel__label">{LOOK_FOR_GROUP}</p>
      <div className="join-group-panel__actions">{children}</div>
    </section>
  )
}

function FifoQueueActions({ queueState, busy, disabled, onJoin, onLeave, prominent = false }) {
  const size = prominent ? 'lg' : 'default'
  const width = prominent ? 'w-full' : undefined
  if (queueState === 'waiting') {
    return (
      <div className="game-list-actions game-list-actions--stack">
        <p className="queue-status-line">{LOOKING_FOR_GROUP}</p>
        <Button size={size} className={width} onClick={onLeave} disabled={busy}>
          {STOP_LOOKING}
        </Button>
      </div>
    )
  }

  return (
    <div className="game-list-actions">
      <Button size={size} className={width} onClick={() => onJoin()} disabled={busy || disabled}>
        {busy ? LOOKING_FOR_GROUP : LOOK_FOR_GROUP}
      </Button>
    </div>
  )
}

export default function GameQueueActions({
  joinOptions,
  queueState,
  joinUrl,
  busy,
  selectedQueuePath = '',
  onJoin,
  onLeave,
  disabled = false,
  prominent = false,
  solo = false,
}) {
  const size = prominent ? 'lg' : 'default'
  const width = prominent ? 'w-full' : undefined

  if (queueState === 'matched' && joinUrl) {
    return (
      <div className="game-list-actions">
        {/* A raw <a> on purpose (JQ-72): this is a button that navigates, so Button
            wears the styling and the anchor keeps real href semantics. */}
        <Button asChild size={size} className={width}>
          <a href={joinUrl}>{LAUNCH_GAME}</a>
        </Button>
        {solo ? null : (
          <Button variant="outline" size={size} className={width} onClick={onLeave} disabled={busy}>
            {LEAVE_MATCH}
          </Button>
        )}
      </div>
    )
  }

  if (solo) {
    return (
      <div className="game-list-actions">
        <Button size={size} className={width} onClick={() => onJoin()} disabled={busy || disabled}>
          {busy ? STARTING_SOLO : PLAY_SOLO}
        </Button>
      </div>
    )
  }

  const compositionPaths =
    joinOptions?.kind === 'composition'
      ? joinOptions.paths.filter((path) => (pathOption(path).queuePath?.trim() ?? '') !== '')
      : []

  const selectedDisplayName = compositionPaths
    .map(pathOption)
    .find(({ queuePath }) => queuePath === selectedQueuePath)?.displayName

  if (compositionPaths.length > 0) {
    const alternatePaths = compositionPaths.filter(
      (path) => pathOption(path).queuePath !== selectedQueuePath,
    )

    return (
      <JoinGroupPanel prominent={prominent}>
        {queueState === 'waiting' ? (
          <>
            <p className="queue-status-line">
              {selectedQueuePath
                ? waitingAsRoleLine(selectedDisplayName || selectedQueuePath)
                : LOOKING_FOR_GROUP}
            </p>
            {alternatePaths.map((path) => (
              <JoinPathButton
                key={pathOption(path).queuePath}
                path={path}
                busy={busy}
                disabled={disabled}
                onJoin={onJoin}
                prominent={prominent}
              />
            ))}
            <Button size={size} className={width} onClick={onLeave} disabled={busy}>
              {STOP_LOOKING}
            </Button>
          </>
        ) : (
          compositionPaths.map((path) => (
            <JoinPathButton
              key={pathOption(path).queuePath}
              path={path}
              busy={busy}
              disabled={disabled}
              onJoin={onJoin}
              prominent={prominent}
            />
          ))
        )}
      </JoinGroupPanel>
    )
  }

  return (
    <FifoQueueActions
      queueState={queueState}
      busy={busy}
      disabled={disabled}
      onJoin={onJoin}
      onLeave={onLeave}
      prominent={prominent}
    />
  )
}
