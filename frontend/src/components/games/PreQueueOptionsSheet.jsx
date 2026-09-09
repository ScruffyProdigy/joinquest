import { useEffect, useMemo, useState } from 'react'
import {
  LOOK_FOR_GROUP,
  LOOKING_FOR_GROUP,
  OPTIONS_UNAVAILABLE,
  optionsChosenCount,
} from '../../lib/playerCopy'
import ModeRequirement from './ModeRequirement'
import { Button } from '../ui/button'

/**
 * Picks per group, keyed by group key, as an ordered array of choice ids.
 *
 * Kept as an object of arrays rather than a flat list because the bounds are
 * per group: a mode can ask for exactly two helpers and exactly one deck.
 */
function togglePick(picks, group, choiceId) {
  const current = picks[group.key] ?? []
  if (current.includes(choiceId)) {
    return { ...picks, [group.key]: current.filter((id) => id !== choiceId) }
  }
  if (current.length >= group.max) {
    // At the cap, another tap replaces the oldest pick rather than doing
    // nothing — silently ignoring a tap reads as a broken button.
    return { ...picks, [group.key]: [...current.slice(1), choiceId] }
  }
  return { ...picks, [group.key]: [...current, choiceId] }
}

export function picksSatisfyGroups(groups, picks) {
  return groups.every((group) => {
    const count = (picks[group.key] ?? []).length
    return count >= group.min && count <= group.max
  })
}

export function selectionsFromPicks(groups, picks) {
  return groups
    .map((group) => ({ groupKey: group.key, optionIds: picks[group.key] ?? [] }))
    .filter((selection) => selection.optionIds.length > 0)
}

function groupCounter(group, picked) {
  if (group.min === group.max) {
    return `${picked} / ${group.max}`
  }
  return `${picked} of ${group.min}–${group.max}`
}

function OptionCard({ choice, selected, onToggle }) {
  const locked = Boolean(choice.locked)
  const classNames = [
    'pre-queue-option',
    selected ? 'pre-queue-option--selected' : '',
    locked ? 'pre-queue-option--locked' : '',
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <button
      type="button"
      className={classNames}
      onClick={() => !locked && onToggle(choice.id)}
      disabled={locked}
      aria-pressed={selected}
    >
      {locked ? (
        <span className="pre-queue-option__lock" aria-hidden="true">
          🔒
        </span>
      ) : null}
      <span className="pre-queue-option__name">{choice.label}</span>
      {choice.description ? (
        <span className="pre-queue-option__description">{choice.description}</span>
      ) : null}
      {/* A locked choice stays on the board showing what it would take, rather
          than vanishing — the player can see what they are working toward. */}
      {locked && choice.requirement ? (
        <span className="pre-queue-option__requirement">
          <ModeRequirement requirement={choice.requirement} />
        </span>
      ) : null}
    </button>
  )
}

export default function PreQueueOptionsSheet({
  open,
  gameName,
  modeName,
  groups = [],
  queueOptions,
  busy = false,
  error = null,
  onConfirm,
  onClose,
}) {
  const [picks, setPicks] = useState({})

  // Reopening starts clean: a stale pick from a previous visit could name an
  // option this player no longer has.
  useEffect(() => {
    if (open) {
      setPicks({})
    }
  }, [open])

  const rosterByGroup = useMemo(() => {
    const byKey = {}
    for (const group of queueOptions?.groups ?? []) {
      byKey[group.key] = group.choices ?? []
    }
    return byKey
  }, [queueOptions])

  if (!open) {
    return null
  }

  const available = queueOptions?.available !== false
  const ready = available && picksSatisfyGroups(groups, picks)

  return (
    <div className="pre-queue-backdrop" role="presentation" onClick={onClose}>
      <section
        className="pre-queue-sheet"
        role="dialog"
        aria-modal="true"
        aria-label={groups[0]?.label ?? 'Choose your options'}
        onClick={(event) => event.stopPropagation()}
      >
        <header className="pre-queue-sheet__header">
          <div>
            <p className="pre-queue-sheet__game">{gameName}</p>
            <h2 className="pre-queue-sheet__title">{modeName}</h2>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} aria-label="Close">
            ✕
          </Button>
        </header>

        <div className="pre-queue-sheet__body">
          {!available ? (
            <p className="pre-queue-sheet__unavailable">
              {queueOptions?.unavailableReason || OPTIONS_UNAVAILABLE}
            </p>
          ) : (
            groups.map((group) => {
              const picked = picks[group.key] ?? []
              return (
                <section key={group.key} className="pre-queue-group">
                  <div className="pre-queue-group__head">
                    <h3 className="pre-queue-group__label">{group.label}</h3>
                    <span className="pre-queue-group__count">{groupCounter(group, picked.length)}</span>
                  </div>
                  <div className="pre-queue-group__grid">
                    {(rosterByGroup[group.key] ?? []).map((choice) => (
                      <OptionCard
                        key={choice.id}
                        choice={choice}
                        selected={picked.includes(choice.id)}
                        onToggle={(id) => setPicks((prev) => togglePick(prev, group, id))}
                      />
                    ))}
                  </div>
                </section>
              )
            })
          )}
          {error ? <p className="pre-queue-sheet__error">{error}</p> : null}
        </div>

        <footer className="pre-queue-sheet__footer">
          <span className="pre-queue-sheet__summary">{optionsChosenCount(groups, picks)}</span>
          <Button
            disabled={!ready || busy}
            onClick={() => onConfirm(selectionsFromPicks(groups, picks))}
          >
            {busy ? LOOKING_FOR_GROUP : LOOK_FOR_GROUP}
          </Button>
        </footer>
      </section>
    </div>
  )
}
