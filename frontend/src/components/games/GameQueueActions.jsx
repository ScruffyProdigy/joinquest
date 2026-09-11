import { ZapIcon } from 'lucide-react'
import {
  FINDING_PLAYERS,
  JUMP_IN,
  LAUNCH_GAME,
  LEAVE_MATCH,
  PLAY_SOLO,
  STARTING_SOLO,
  STOP_FINDING,
  joinAsLabel,
  waitingAsRoleLine,
} from '../../lib/playerCopy'
import { navigateToLaunchUrl } from '../../lib/launch'
import { Button } from '../ui/button'

/**
 * The prototype leads its primary CTA with a lucide `zap`. `buttonVariants`
 * already sizes a slotted svg (`[&_svg]:size-4`), so the icon needs no styling
 * of its own -- only hiding, so the button's accessible name stays the label.
 */
function JumpInIcon() {
  return <ZapIcon aria-hidden="true" focusable="false" />
}

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
      size={prominent ? 'default' : 'sm'}
      onClick={() => onJoin(queuePath)}
      disabled={busy || disabled}
    >
      {busy ? FINDING_PLAYERS : joinAsLabel(displayName)}
    </Button>
  )
}

function JoinGroupPanel({ children, prominent = false }) {
  return (
    <section
      className={`join-group-panel${prominent ? ' join-group-panel--prominent' : ''}`}
      aria-label={JUMP_IN}
    >
      <p className="join-group-panel__label">{JUMP_IN}</p>
      <div className="join-group-panel__actions">{children}</div>
    </section>
  )
}

function FifoQueueActions({ queueState, busy, disabled, onJoin, onLeave, prominent = false }) {
  const size = prominent ? 'default' : 'sm'
  if (queueState === 'waiting') {
    return (
      <div className="game-list-actions game-list-actions--stack">
        <p className="queue-status-line">{FINDING_PLAYERS}</p>
        <Button size={size} onClick={onLeave} disabled={busy}>
          {STOP_FINDING}
        </Button>
      </div>
    )
  }

  return (
    <div className="game-list-actions">
      <Button size={size} onClick={() => onJoin()} disabled={busy || disabled}>
        <JumpInIcon />
        {busy ? FINDING_PLAYERS : JUMP_IN}
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
  const size = prominent ? 'default' : 'sm'

  if (queueState === 'matched' && joinUrl) {
    return (
      <div className="game-list-actions">
        {/* A button rather than a link: the launch URL carries a token, which has no
            business sitting in the DOM waiting to be copied or to go stale (JQ-261). */}
        <Button type="button" size={size} onClick={() => navigateToLaunchUrl(joinUrl)}>
          {LAUNCH_GAME}
        </Button>
        {solo ? null : (
          <Button variant="outline" size={size} onClick={onLeave} disabled={busy}>
            {LEAVE_MATCH}
          </Button>
        )}
      </div>
    )
  }

  if (solo) {
    return (
      <div className="game-list-actions">
        <Button size={size} onClick={() => onJoin()} disabled={busy || disabled}>
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
                : FINDING_PLAYERS}
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
            <Button size={size} onClick={onLeave} disabled={busy}>
              {STOP_FINDING}
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
