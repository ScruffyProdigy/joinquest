import {
  countSeatedInGroup,
  firstOpenSeatKey,
  groupSeatSlotsForDisplay,
  isPooledRoleGroup,
  seatSectionTitle,
} from '../../lib/tables'
import { accentBaseFor } from '../../lib/gameAccent'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { Button } from '../ui/button'
import { Card, CardContent } from '../ui/card'

function SeatGroup({ title, slots, accent, userId, busy, onClaim, onLeaveSeat }) {
  const seatedHere = slots.some((slot) => slot.user?.id && slot.user.id === userId)
  const openSeatKey = firstOpenSeatKey(slots)
  const pooled = isPooledRoleGroup(slots)

  return (
    <div className="flex items-center justify-between gap-3 border-t px-4 py-3 first:border-t-0">
      <p className="text-sm font-semibold">
        {title} <span className="font-normal text-muted-foreground">× {slots.length}</span>
      </p>
      <div className="flex items-center gap-3">
        <div className="flex gap-1">
          {slots.map((slot) =>
            slot.user ? (
              <PlayerAvatar key={slot.seatKey} user={slot.user} size="sm" />
            ) : (
              <span
                key={slot.seatKey}
                className="box-border size-8 rounded-full border-2 border-dashed"
                style={{ borderColor: `color-mix(in oklab, ${accent} 45%, transparent)` }}
                aria-label="Open seat"
              />
            ),
          )}
        </div>
        {seatedHere ? (
          <Button type="button" variant="secondary" size="sm" disabled={busy} onClick={onLeaveSeat}>
            Leave
          </Button>
        ) : openSeatKey ? (
          <Button
            type="button"
            size="sm"
            disabled={busy}
            onClick={() => onClaim(pooled ? firstOpenSeatKey(slots) : openSeatKey)}
          >
            Claim
          </Button>
        ) : null}
      </div>
    </div>
  )
}

export default function GroupSeatList({ table, userId, busy, onClaim, onLeaveSeat }) {
  const accent = accentBaseFor(table?.game?.slug, table?.game?.accentColor)
  const slots = table?.seatSlots ?? []
  const layout = groupSeatSlotsForDisplay(slots)
  const seated = countSeatedInGroup(slots)

  let groups
  if (layout.kind === 'roles') {
    groups = layout.roles
  } else if (layout.kind === 'teams') {
    groups = layout.teams.map(([prefix, teamSlots]) => [prefix.replace('-', ' '), teamSlots])
  } else {
    groups = layout.slots.length ? [[seatSectionTitle(layout.slots), layout.slots]] : []
  }

  return (
    <section className="px-4 pt-4" aria-label="Players">
      <Card className="gap-0 py-0">
        <CardContent className="px-0">
          <div className="flex items-center justify-between px-4 py-3">
            <p className="flex items-center gap-3 text-sm font-semibold">
              <span
                aria-hidden="true"
                className="size-2.5 shrink-0 rounded-full"
                style={{ background: accent }}
              />
              Players
            </p>
            <p className="text-sm text-muted-foreground">
              {seated}/{slots.length}
            </p>
          </div>
          {groups.map(([title, groupSlots]) => (
            <SeatGroup
              key={title}
              title={title}
              slots={groupSlots}
              accent={accent}
              userId={userId}
              busy={busy}
              onClaim={onClaim}
              onLeaveSeat={onLeaveSeat}
            />
          ))}
        </CardContent>
      </Card>
    </section>
  )
}
