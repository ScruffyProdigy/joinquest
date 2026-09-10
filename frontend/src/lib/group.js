import { displayName, isKing, mySeatKeyOnTable, seatSectionTitle } from './tables'

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

/**
 * The roles a table is still short of. `formingGaps` is the API's answer and is
 * preferred, but it is empty for a private table nobody is queueing to fill — and the
 * header still has to name what is missing rather than count seats. The open slots
 * carry it: a mode that names no roles leaves them numbered, and `seatSectionTitle`
 * turns that into "Player".
 */
function missingRoleNames(table) {
  const gaps = (table?.formingGaps ?? [])
    .filter((gap) => (gap.needed ?? 0) > (gap.assigned ?? 0))
    .map((gap) => gap.displayName || gap.queuePath)
    .filter(Boolean)
  if (gaps.length > 0) {
    return gaps
  }

  const names = []
  for (const slot of table?.seatSlots ?? []) {
    if (slot.user) {
      continue
    }
    const name = seatSectionTitle([slot])
    if (!names.includes(name)) {
      names.push(name)
    }
  }
  return names
}

/** Header status: the roles still missing, else readiness. */
export function groupStatusLine(table) {
  const { total, seated } = seatCounts(table)
  const missing = missingRoleNames(table)

  if (missing.length > 0) {
    return `Still need: ${missing.join(', ')}`
  }
  if (total > 0 && seated >= total) {
    return `${seated} of ${total} seats · ready to start`
  }
  return `${seated} of ${total} seats`
}

/**
 * Everyone still to decide, as `{ user, status }` — the prototype's "Picking a seat".
 * Two populations in one list: room members who hold no seat (`here`), and the previous
 * match's players who have not answered (`awaiting`) or have declined (`out`).
 *
 * A roster entry beats room membership, and that is the one deliberate departure from the
 * prototype. There a returner flips `awaiting → in-lobby` when they *arrive*, because
 * arrival is what a prototype can observe. Production cannot reuse that: a room-table
 * group never leaves the room, so membership would read every one of them as here while
 * none had answered. `Table.regroupRoster` already carries the real signal.
 *
 * The viewer is the exception — they are demonstrably back, so they are never awaiting.
 *
 * Nothing here keys on `seatKey`. A mode with no seat template still lists its pending
 * players, and more pending players than seats drops none of them.
 */
export function playersPickingASeat(room, table, viewerId = null) {
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

  const roster = new Map()
  for (const entry of table?.regroupRoster ?? []) {
    if (entry?.user?.id && !roster.has(entry.user.id)) {
      roster.set(entry.user.id, entry.regroup)
    }
  }

  const viewer = []
  const here = []
  const awaiting = []
  const out = []
  const listed = new Set()

  function add(user) {
    const id = user?.id
    // A seated player is on the Players card already; a second row for them there and
    // here reads as two people.
    if (!id || seated.has(id) || listed.has(id)) {
      return
    }
    listed.add(id)
    if (id === viewerId) {
      viewer.push({ user, status: 'here' })
      return
    }
    const regroup = roster.get(id)
    if (regroup === 'PENDING') {
      awaiting.push({ user, status: 'awaiting' })
    } else if (regroup === 'OUT') {
      out.push({ user, status: 'out' })
    } else {
      here.push({ user, status: 'here' })
    }
  }

  for (const member of room?.members ?? []) {
    add(member)
  }
  for (const entry of table?.regroupRoster ?? []) {
    add(entry?.user)
  }

  return [...viewer, ...here, ...awaiting, ...out]
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
