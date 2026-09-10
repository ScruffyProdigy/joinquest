import { useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { gameAxisChips, gameIconUrl } from '../../lib/gameCard'
import { accentColorFor } from '../../lib/gameAccent'
import {
  DISCARD,
  formatFormingGapsFromLobbyLine,
  KING_LABEL,
  JUMP_IN,
  START_GAME,
} from '../../lib/playerCopy'
import {
  countSeatedInGroup,
  displayName,
  enrichTableSeats,
  firstOpenSeatKey,
  formatGroupSeatCaption,
  groupSeatSlotsForDisplay,
  isKing,
  isPooledRoleGroup,
  mySeatDisplayName,
  mySeatKeyOnTable,
  queuePathMeta,
  seatLabelInSection,
} from '../../lib/tables'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { Card } from '../ui/card'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter } from '../ui/sheet'
import { cn } from '../../lib/utils'

function SeatRow({ slot, seatLabel, sectionTitle, groupSlots, mySeat, userId, currentUser, kingUserId, busy, onRequestSit }) {
  const taken = Boolean(slot.user)
  const isMine = mySeat === slot.seatKey || slot.user?.id === userId
  const showOccupant = taken || isMine
  const occupantUser = isMine ? currentUser ?? slot.user : slot.user
  const occupantLabel = isMine ? 'You' : displayName(slot.user)
  const isTableKing = Boolean(kingUserId && occupantUser?.id === kingUserId)

  if (!taken) {
    return (
      <li className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-background/30 px-3 py-2">
        <div className="flex items-center gap-3">
          {seatLabel ? <span className="text-sm text-muted-foreground">{seatLabel}</span> : null}
        </div>
        <Button
          type="button"
          size="sm"
          variant="secondary"
          disabled={busy}
          onClick={() => onRequestSit(slot, sectionTitle, groupSlots)}
        >
          Sit
        </Button>
      </li>
    )
  }

  return (
    <li
      className={cn(
        'flex items-center gap-3 rounded-lg border px-3 py-2',
        isMine ? 'border-primary/60 bg-primary/10' : 'border-border/60 bg-background/30',
      )}
    >
      {seatLabel ? <span className="text-sm text-muted-foreground">{seatLabel}</span> : null}
      {showOccupant ? (
        <div className="flex items-center gap-2" title={occupantLabel}>
          <PlayerAvatar user={occupantUser} size="md" ring={isTableKing ? 'king' : undefined} />
          <span className="text-sm text-foreground">{occupantLabel}</span>
        </div>
      ) : null}
    </li>
  )
}

function SeatSection({ title, slots, mode, mySeat, userId, currentUser, kingUserId, busy, onRequestSit }) {
  const queuePath = slots[0]?.queuePath
  const meta = queuePathMeta(mode, queuePath)
  const seatedCount = countSeatedInGroup(slots)
  const pooled = isPooledRoleGroup(slots)
  const mineInGroup = slots.some((slot) => slot.seatKey === mySeat || slot.user?.id === userId)
  const openSeatKey = firstOpenSeatKey(slots)
  const rowProps = { sectionTitle: title, groupSlots: slots, mySeat, userId, currentUser, kingUserId, busy, onRequestSit }

  if (pooled) {
    const occupants = slots.filter((slot) => slot.user || slot.seatKey === mySeat)
    const openSlot = slots.find((slot) => slot.seatKey === openSeatKey)
    return (
      <div className="flex flex-col gap-2">
        <div className="flex items-baseline justify-between">
          <h4 className="text-sm font-semibold text-foreground">{title}</h4>
          <p className="text-xs text-muted-foreground">{formatGroupSeatCaption(seatedCount, meta)}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex flex-wrap gap-3" aria-label={`${title} seats`}>
            {occupants.length > 0 ? (
              occupants.map((slot) => {
                const isMine = mySeat === slot.seatKey || slot.user?.id === userId
                const occupantUser = isMine ? currentUser ?? slot.user : slot.user
                const occupantLabel = isMine ? 'You' : displayName(slot.user)
                const isTableKing = Boolean(kingUserId && occupantUser?.id === kingUserId)
                const seatBadge = seatLabelInSection(slot, title)
                const titleText = seatBadge ? `${seatBadge} · ${occupantLabel}` : occupantLabel
                return (
                  <div
                    key={slot.seatKey}
                    className={cn('flex flex-col items-center gap-1 text-xs', isMine ? 'text-primary' : 'text-muted-foreground')}
                    title={titleText}
                  >
                    {seatBadge ? <span className="text-[10px] uppercase tracking-wide">{seatBadge}</span> : null}
                    <PlayerAvatar user={occupantUser} size="md" ring={isTableKing ? 'king' : undefined} />
                    <span>{occupantLabel}</span>
                  </div>
                )
              })
            ) : (
              <span className="text-xs text-muted-foreground">No one seated yet</span>
            )}
          </div>
          {!mineInGroup && openSlot ? (
            <Button type="button" size="sm" variant="secondary" disabled={busy} onClick={() => onRequestSit(openSlot, title, slots)}>
              Sit
            </Button>
          ) : null}
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-baseline justify-between">
        <h4 className="text-sm font-semibold text-foreground">{title}</h4>
        {meta ? <p className="text-xs text-muted-foreground">{formatGroupSeatCaption(seatedCount, meta)}</p> : null}
      </div>
      <ul className="flex flex-col gap-2">
        {slots.map((slot) => (
          <SeatRow key={slot.seatKey} slot={slot} seatLabel={seatLabelInSection(slot, title)} {...rowProps} />
        ))}
      </ul>
    </div>
  )
}

// Reachable only through RoomPanel, which JQ-206 unmounted — see the note
// there for why both are kept rather than deleted.
export default function TableCard({ table, busy, onSit, onLeave, onStart, onLookForGroup, onDiscard }) {
  const { user } = useAuth()
  const [pickerSlot, setPickerSlot] = useState(null)
  const enriched = enrichTableSeats(table)
  const mySeat = mySeatKeyOnTable(enriched, user?.id)
  const mySeatLabel = mySeatDisplayName(enriched, user?.id)
  const king = isKing(enriched, user?.id)
  const kingUserId = enriched.king?.id
  const layout = groupSeatSlotsForDisplay(enriched.seatSlots ?? [])
  const seatedCount = enriched.seats?.length ?? 0
  const gapsLine = formatFormingGapsFromLobbyLine(enriched.formingGaps)
  const game = enriched.game
  const accent = accentColorFor(game?.slug || game?.id || 'table', game?.accentColor)
  const catalogTags = gameAxisChips(game, enriched.mode)
  const catalogBlurb = game?.shortDescription?.trim() || ''
  let catalogIcon = null
  if (game?.iconUrl?.trim()) {
    try {
      catalogIcon = gameIconUrl(game)
    } catch {
      catalogIcon = null
    }
  }

  function handleRequestSit(slot, sectionTitle, groupSlots) {
    setPickerSlot({ slot, sectionTitle, groupSlots })
  }

  function confirmSit() {
    if (!pickerSlot) {
      return
    }
    onSit(pickerSlot.slot.seatKey)
    setPickerSlot(null)
  }

  const seatRowProps = {
    mode: enriched.mode,
    mySeat,
    userId: user?.id,
    currentUser: user,
    kingUserId,
    busy,
    onRequestSit: handleRequestSit,
  }

  const pickerSlotOccupants = (pickerSlot?.groupSlots ?? []).filter((slot) => slot.user)

  return (
    <Card className="gap-4 overflow-hidden border py-4" style={{ background: accent.cardBg, borderColor: accent.border }}>
      <header className="flex gap-3 px-4">
        {catalogIcon ? (
          <img className="h-16 w-16 shrink-0 rounded-lg object-cover" src={catalogIcon} alt="" width={64} height={64} loading="lazy" />
        ) : null}
        <div className="flex min-w-0 flex-col gap-1">
          <h3 className="font-heading text-base font-semibold text-foreground">
            {game?.name} · {enriched.mode?.displayName}
          </h3>
          {catalogBlurb ? <p className="text-sm text-muted-foreground">{catalogBlurb}</p> : null}
          {catalogTags.length > 0 ? (
            <ul className="flex flex-wrap gap-1" aria-label="Game genre, format and difficulty">
              {catalogTags.map((tag) => (
                <li key={tag}>
                  <Badge variant="secondary">{tag}</Badge>
                </li>
              ))}
            </ul>
          ) : null}
          <p className="text-xs text-muted-foreground">
            {seatedCount} seated
            {enriched.king ? ` · ${KING_LABEL}: ${displayName(enriched.king)}` : ''}
            {mySeatLabel ? ` · Your seat: ${mySeatLabel}` : ''}
          </p>
        </div>
      </header>

      <div
        className={cn(
          'flex flex-col gap-4 px-4',
          (layout.kind === 'teams' || layout.kind === 'roles') && 'sm:grid sm:grid-cols-2 sm:gap-x-6',
        )}
      >
        {layout.kind === 'teams' ? (
          <>
            {layout.teams.map(([prefix, slots]) => (
              <SeatSection key={prefix} title={prefix.replace('-', ' ')} slots={slots} {...seatRowProps} />
            ))}
            {layout.ungrouped.length > 0 ? <SeatSection title="Seats" slots={layout.ungrouped} {...seatRowProps} /> : null}
          </>
        ) : null}

        {layout.kind === 'roles' ? (
          <>
            {layout.roles.map(([title, slots]) => (
              <SeatSection key={title} title={title} slots={slots} {...seatRowProps} />
            ))}
            {layout.noPath.length > 0 ? <SeatSection title="Seats" slots={layout.noPath} {...seatRowProps} /> : null}
          </>
        ) : null}

        {layout.kind === 'flat' ? (
          isPooledRoleGroup(layout.slots) ? (
            <SeatSection title="Players" slots={layout.slots} {...seatRowProps} />
          ) : (
            <ul className="flex flex-col gap-2">
              {layout.slots.map((slot) => (
                <SeatRow
                  key={slot.seatKey}
                  slot={slot}
                  seatLabel={seatLabelInSection(slot, null)}
                  sectionTitle={null}
                  groupSlots={layout.slots}
                  {...seatRowProps}
                />
              ))}
            </ul>
          )
        ) : null}
      </div>

      {gapsLine ? <p className="px-4 text-xs text-muted-foreground">{gapsLine}</p> : null}

      <div className="flex flex-wrap gap-2 px-4">
        {mySeat ? (
          <Button type="button" variant="secondary" disabled={busy} onClick={onLeave}>
            Leave seat
          </Button>
        ) : null}
        {king && enriched.canStart ? (
          <Button type="button" disabled={busy} onClick={onStart}>
            {START_GAME}
          </Button>
        ) : null}
        {king
          ? enriched.lookForGroupOptions
              ?.filter((opt) => opt.visible)
              .map((opt) => (
                <Button
                  key={opt.queueId}
                  type="button"
                  disabled={busy || !opt.enabled || enriched.backfillActive}
                  onClick={() => onLookForGroup?.(opt.queueId)}
                >
                  {JUMP_IN} ({opt.queueName})
                </Button>
              ))
          : null}
        {enriched.canDiscard ? (
          <Button type="button" variant="destructive" disabled={busy} onClick={onDiscard}>
            {DISCARD}
          </Button>
        ) : null}
      </div>

      <Sheet
        open={Boolean(pickerSlot)}
        onOpenChange={(next) => {
          if (!next) {
            setPickerSlot(null)
          }
        }}
      >
        <SheetContent side="bottom">
          <SheetHeader>
            <SheetTitle>Choose your seat</SheetTitle>
            {pickerSlot?.sectionTitle ? (
              <p className="text-sm text-muted-foreground">{pickerSlot.sectionTitle}</p>
            ) : null}
          </SheetHeader>
          <div className="flex flex-wrap gap-4 px-4">
            {pickerSlotOccupants.length > 0 ? (
              pickerSlotOccupants.map((slot) => (
                <div key={slot.seatKey} className="flex flex-col items-center gap-1 text-xs text-muted-foreground">
                  <PlayerAvatar user={slot.user} size="md" />
                  <span>{displayName(slot.user)}</span>
                </div>
              ))
            ) : (
              <span className="text-xs text-muted-foreground">No one else here yet</span>
            )}
          </div>
          {/* Role/seat description text lands here once JQ-42 adds it — intentionally empty so that ticket doesn't need a second visual pass. */}
          <SheetFooter>
            <Button type="button" disabled={busy} onClick={confirmSit}>
              Confirm seat
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </Card>
  )
}
