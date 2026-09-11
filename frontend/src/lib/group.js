import { displayName, isKing, mySeatKeyOnTable, seatSectionTitle } from './tables'
import {
  formatGroupFillNeedLine,
  GROUP_FIND_MATCH,
  GROUP_FIND_MATCH_HINT,
  GROUP_FINDING_MATCH,
  GROUP_STARTING,
  STOP_FINDING,
} from './playerCopy'

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
  // `needed` is already the remainder the API computed (PlayersToStart minus assigned),
  // so it is the whole test — comparing it against `assigned` drops a path that still
  // needs exactly as many as it has.
  const gaps = (table?.formingGaps ?? [])
    .filter((gap) => (gap.needed ?? 0) > 0)
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

  // `away` rides alongside `status` rather than becoming one of its values (JQ-265). They
  // answer different questions — status is what this player has decided about the next
  // match, away is whether we still believe they are at their device — and a player can
  // be both at once: someone whose regroup answer is still pending and whose phone died
  // is awaiting AND away, and collapsing that into one field would have to drop one of
  // the two facts.
  function add(user, away = false) {
    const id = user?.id
    // A seated player is on the Players card already; a second row for them there and
    // here reads as two people.
    if (!id || seated.has(id) || listed.has(id)) {
      return
    }
    listed.add(id)
    if (id === viewerId) {
      viewer.push({ user, status: 'here', away })
      return
    }
    const regroup = roster.get(id)
    if (regroup === 'PENDING') {
      awaiting.push({ user, status: 'awaiting', away })
    } else if (regroup === 'OUT') {
      out.push({ user, status: 'out', away })
    } else {
      here.push({ user, status: 'here', away })
    }
  }

  // `disconnected` is the API's reading; `away` is the word these entries and the cards
  // that render them use, because it is the word a player understands (and because
  // JQ-179's idle signal will feed the same display without being a disconnect).
  for (const member of room?.members ?? []) {
    add(member?.user, member?.disconnected ?? false)
  }
  // The regroup roster carries no presence of its own — it is a record of answers, not of
  // sockets. Anyone on it who is also a room member was already added above with their
  // real reading, and `listed` keeps this pass from overwriting it; anyone who is not a
  // member has left the room, which is a stronger statement than away.
  for (const entry of table?.regroupRoster ?? []) {
    add(entry?.user)
  }

  return [...viewer, ...here, ...awaiting, ...out]
}

/** Every seat the mode declares is taken. */
function isFull(table) {
  const slots = table?.seatSlots ?? []
  return slots.length > 0 && slots.every((slot) => slot.user)
}

/**
 * The queue a request for the rest of the match would go to, or null when there is none
 * to ask. `lookForGroupOptions` is the API's own answer to both questions — whether this
 * table has room left (`visible`) and whether it is free to ask right now (`enabled`).
 */
export function findMatchQueueId(table) {
  const option = (table?.lookForGroupOptions ?? []).find((opt) => opt.visible && opt.enabled)
  return option?.queueId ?? null
}

/**
 * The sticky bottom control.
 *
 * One decision sits behind every branch here: when does the group stop waiting for the
 * people who are not in it yet. Filling the remaining seats from the lobby and starting
 * short-handed are two spellings of it rather than two powers — for a mode whose minimum
 * is its full complement, filling *is* how a partial group starts at all — and it ends
 * the wait for anyone still on their way either way, selling their seat instead of
 * playing without them. So it keeps one owner, the king, exactly as starting early always
 * did (JQ-137).
 *
 * Everyone else is told what is happening and that they need do nothing. The page still
 * never says "king" — it names the person, the way the prototype does.
 */
export function groupCtaState(table, userId) {
  if (!mySeatKeyOnTable(table, userId)) {
    return { kind: 'claim', label: 'Claim a seat to join' }
  }

  const king = isKing(table, userId)

  if (table?.backfillActive) {
    return {
      kind: 'filling',
      label: GROUP_FINDING_MATCH,
      detail: formatGroupFillNeedLine(table.formingGaps),
      hint: GROUP_FIND_MATCH_HINT,
      // Withdrawing puts the whole group back to waiting, so it belongs to whoever
      // committed them. Everybody else keeps the line telling them to sit tight.
      cancelLabel: king ? STOP_FINDING : null,
    }
  }

  // A full table starts itself, so there is nothing here to press — only the moment to
  // announce. This is what retires the king-gated Start for the friends-only case: the
  // button is not disabled, it is gone, because waiting for one person to notice a full
  // table was never the point of having a king.
  if (isFull(table)) {
    return { kind: 'starting', label: GROUP_STARTING }
  }

  const queueId = findMatchQueueId(table)

  if (king) {
    if (queueId) {
      return {
        kind: 'find',
        label: GROUP_FIND_MATCH,
        hint: GROUP_FIND_MATCH_HINT,
        detail: groupStatusLine(table),
        queueId,
        // Offered alongside, not instead: a group already past the mode's minimum may
        // prefer to play short now rather than wait for strangers to arrive. Same
        // decision, different answer about who fills the gap.
        startLabel: table?.canStart ? 'Start game' : null,
      }
    }
    if (table?.canStart) {
      return { kind: 'start', label: 'Start game' }
    }
    return { kind: 'blocked', label: groupStatusLine(table) }
  }

  // Not the king. Anything that could end the wait is theirs to trigger, so say who and
  // promise the ride in rather than offering a button that would be refused.
  if (table?.king && (queueId || table?.canStart)) {
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
