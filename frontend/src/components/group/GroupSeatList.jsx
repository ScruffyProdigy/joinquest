import {
  countSeatedInGroup,
  firstOpenSeatKey,
  groupSeatSlotsForDisplay,
  isPooledRoleGroup,
} from '../../lib/tables'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { Button } from '../ui/button'
import { Card, CardContent } from '../ui/card'

function sectionTitle(slots, fallback) {
  return slots[0]?.displayName?.split(' · ')[0]?.trim() || fallback
}

function SeatGroup({ title, slots, userId, busy, onClaim, onLeaveSeat }) {
  const seatedHere = slots.some((slot) => slot.user?.id && slot.user.id === userId)
  const openSeatKey = firstOpenSeatKey(slots)
  const pooled = isPooledRoleGroup(slots)

  return (
    <div className="flex items-center justify-between gap-3 border-t px-4 py-3 first:border-t-0">
      <p className="text-sm font-medium">
        {title} <span className="text-muted-foreground">× {slots.length}</span>
      </p>
      <div className="flex items-center gap-2">
        <div className="flex -space-x-2">
          {slots.map((slot) =>
            slot.user ? (
              <PlayerAvatar key={slot.seatKey} user={slot.user} size="sm" />
            ) : (
              <span
                key={slot.seatKey}
                className="size-8 rounded-full border border-dashed border-muted-foreground/40"
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
  const slots = table?.seatSlots ?? []
  const layout = groupSeatSlotsForDisplay(slots)
  const seated = countSeatedInGroup(slots)

  let groups
  if (layout.kind === 'roles') {
    groups = layout.roles
  } else if (layout.kind === 'teams') {
    groups = layout.teams.map(([prefix, teamSlots]) => [prefix.replace('-', ' '), teamSlots])
  } else {
    groups = layout.slots.length ? [[sectionTitle(layout.slots, 'Players'), layout.slots]] : []
  }

  return (
    <section className="px-4 pt-4" aria-label="Players">
      <Card className="gap-0 py-0">
        <CardContent className="px-0">
          <div className="flex items-center justify-between px-4 py-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
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
