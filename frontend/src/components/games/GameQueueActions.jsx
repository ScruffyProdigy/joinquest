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
    <button
      type="button"
      className={`game-list-button${prominent ? ' game-list-button--prominent' : ''}`}
      onClick={() => onJoin(queuePath)}
      disabled={busy || disabled}
    >
      {busy ? LOOKING_FOR_GROUP : joinAsLabel(displayName)}
    </button>
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
  const buttonClass = `game-list-button${prominent ? ' game-list-button--prominent' : ''}`
  if (queueState === 'waiting') {
    return (
      <div className="game-list-actions game-list-actions--stack">
        <p className="queue-status-line">{LOOKING_FOR_GROUP}</p>
        <button type="button" className={buttonClass} onClick={onLeave} disabled={busy}>
          {STOP_LOOKING}
        </button>
      </div>
    )
  }

  return (
    <div className="game-list-actions">
      <button
        type="button"
        className={buttonClass}
        onClick={() => onJoin()}
        disabled={busy || disabled}
      >
        {busy ? LOOKING_FOR_GROUP : LOOK_FOR_GROUP}
      </button>
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
  const buttonClass = `game-list-button${prominent ? ' game-list-button--prominent' : ''}`
  const secondaryClass = `${buttonClass} game-list-button-secondary`

  if (queueState === 'matched' && joinUrl) {
    return (
      <div className="game-list-actions">
        {/* A raw <a> on purpose (JQ-72): this is a button that navigates, styled by
            game-list-button, not a link. */}
        <a className={buttonClass} href={joinUrl}>
          {LAUNCH_GAME}
        </a>
        {solo ? null : (
          <button type="button" className={secondaryClass} onClick={onLeave} disabled={busy}>
            {LEAVE_MATCH}
          </button>
        )}
      </div>
    )
  }

  if (solo) {
    return (
      <div className="game-list-actions">
        <button
          type="button"
          className={buttonClass}
          onClick={() => onJoin()}
          disabled={busy || disabled}
        >
          {busy ? STARTING_SOLO : PLAY_SOLO}
        </button>
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
            <button type="button" className={buttonClass} onClick={onLeave} disabled={busy}>
              {STOP_LOOKING}
            </button>
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
