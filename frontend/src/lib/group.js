import { displayName, isKing, mySeatKeyOnTable } from './tables'

/**
 * The group screen is a presentation over the room+table substrate: one table in a
 * room the player never had to think about (JQ-132). Nothing here is a parallel data
 * model — every value is derived from Table.seatSlots, Table.formingGaps and
 * Room.members as the API already returns them.
 */

export function parseGroupRoute(pathname = window.location.pathname) {
  return /^\/group\/?$/.test(pathname)
}

function seatCounts(table) {
  const slots = table?.seatSlots ?? []
  return { total: slots.length, seated: slots.filter((slot) => slot.user).length }
}

/** Header status: the roles still missing, else readiness. */
export function groupStatusLine(table) {
  const { total, seated } = seatCounts(table)
  const missing = (table?.formingGaps ?? [])
    .filter((gap) => (gap.needed ?? 0) > (gap.assigned ?? 0))
    .map((gap) => gap.displayName || gap.queuePath)
    .filter(Boolean)

  if (missing.length > 0) {
    return `Still need: ${missing.join(', ')}`
  }
  if (total > 0 && seated >= total) {
    return `${seated} of ${total} seats · ready to start`
  }
  return `${seated} of ${total} seats`
}

/** Room members holding no seat on this table — the prototype's "Picking a seat". */
export function playersPickingASeat(room, table) {
  const seated = new Set()
  for (const seat of table?.seats ?? []) {
    if (seat.user?.id) {
      seated.add(seat.user.id)
    }
  }
  for (const slot of table?.seatSlots ?? []) {
    if (slot.user?.id) {
      seated.add(slot.user.id)
    }
  }
  return (room?.members ?? []).filter((member) => member?.id && !seated.has(member.id))
}

/**
 * The sticky bottom control. The king gate is production's, but the page never says
 * "king" — it names the person, the way the prototype does. Auto-start when a table
 * fills is JQ-137, not this.
 */
export function groupCtaState(table, userId) {
  if (!mySeatKeyOnTable(table, userId)) {
    return { kind: 'claim', label: 'Claim a seat to join' }
  }
  if (isKing(table, userId)) {
    if (table?.canStart) {
      return { kind: 'start', label: 'Start game' }
    }
    return { kind: 'blocked', label: groupStatusLine(table) }
  }
  if (table?.king) {
    return {
      kind: 'waiting',
      label: `Waiting for ${displayName(table.king)} to start`,
      hint: "You'll be taken in automatically",
    }
  }
  return { kind: 'blocked', label: groupStatusLine(table) }
}

/**
 * Whether leaving would empty the table. LeaveTable only deletes the seat, so the
 * last player out also discards — otherwise every bounce off the page leaves an
 * abandoned forming table behind in the player's room.
 */
export function isLastSeatedPlayer(table, userId) {
  const seated = new Set()
  for (const seat of table?.seats ?? []) {
    if (seat.user?.id) {
      seated.add(seat.user.id)
    }
  }
  for (const slot of table?.seatSlots ?? []) {
    if (slot.user?.id) {
      seated.add(slot.user.id)
    }
  }
  return seated.size === 1 && seated.has(userId)
}

/**
 * The one table a group page is about. A room may legitimately hold more (someone
 * reached it through the room surfaces), so prefer the table the player sits at and
 * otherwise show the newest.
 */
export function selectGroupTable(room, userId) {
  const tables = room?.tables ?? []
  if (tables.length === 0) {
    return null
  }
  const seated = tables.find((table) => mySeatKeyOnTable(table, userId))
  if (seated) {
    return seated
  }
  return [...tables].sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)))[0] ?? null
}

/**
 * Where an invite-link arrival belongs. One forming table is a group; anything else
 * is a room and keeps the room surfaces, which still handle every case they did before.
 */
export function groupLandingPath(room) {
  return (room?.tables ?? []).length === 1 ? '/group' : null
}
