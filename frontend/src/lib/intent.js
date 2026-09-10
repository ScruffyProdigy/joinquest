import { graphqlRequest } from './graphql'
import { bannerIntentPlayingLine } from './playerCopy'
import { fetchMyTableSeat } from './tables'

const MY_ACTIVE_INTENT_QUERY = `
  query MyActiveIntent {
    myActiveIntent {
      queueId
      gameId
      gameName
      modeName
      seatDisplayName
      status
      queuedCount
      queuePath
      queuePathDisplayName
      joinUrl
      formingGaps {
        queuePath
        displayName
        assigned
        needed
      }
      selectedOptions {
        groupKey
        optionIds
        labels
      }
    }
  }
`

const LEAVE_ACTIVE_GAME = `
  mutation LeaveActiveGame {
    leaveActiveGame
  }
`

const REJOIN_ACTIVE_MATCH = `
  mutation RejoinActiveMatch {
    rejoinActiveMatch
  }
`

export async function fetchMyActiveIntent() {
  const data = await graphqlRequest(MY_ACTIVE_INTENT_QUERY)
  return data.myActiveIntent ?? null
}

export async function leaveActiveGame() {
  const data = await graphqlRequest(LEAVE_ACTIVE_GAME)
  return data.leaveActiveGame === true
}

/**
 * Mints a fresh way back into the match the player is already seated in (JQ-86).
 *
 * Minting happens on click rather than with the banner: the rejoin token is
 * deliberately short-lived, so a URL fetched when the banner rendered would be
 * stale by the time someone who lost the tab came back to use it.
 */
export async function rejoinActiveMatch() {
  const data = await graphqlRequest(REJOIN_ACTIVE_MATCH)
  const url = data?.rejoinActiveMatch
  return typeof url === 'string' && url ? url : null
}

export function hasPlayingIntent(activeIntent) {
  return activeIntent?.status === 'MATCHED'
}

export function hasWaitingIntent(activeIntent) {
  return activeIntent?.status === 'WAITING'
}

export function hasStartedTableSession(activeTableSeat) {
  return activeTableSeat?.status === 'started'
}

export function hasReadyToPlayIntent(activeIntent, activeTableSeat) {
  return hasPlayingIntent(activeIntent) || hasStartedTableSession(activeTableSeat)
}

export function resolveIntentLaunchUrl(activeIntent, activeTableSeat) {
  return activeIntent?.joinUrl || activeTableSeat?.joinUrl || null
}

export function playingIntentTitle(activeIntent, activeTableSeat) {
  if (hasPlayingIntent(activeIntent)) {
    const roleLabel =
      activeIntent.seatDisplayName?.trim() ||
      activeIntent.queuePathDisplayName?.trim() ||
      activeTableSeat?.seatDisplayName?.trim()
    return bannerIntentPlayingLine(activeIntent.gameName, activeIntent.modeName, roleLabel || null)
  }
  if (hasStartedTableSession(activeTableSeat)) {
    const roleLabel = activeTableSeat.seatDisplayName?.trim()
    return bannerIntentPlayingLine(activeTableSeat.gameName, activeTableSeat.modeName, roleLabel || null)
  }
  return ''
}

export function hasFormingTableIntent(activeIntent, activeTableSeat) {
  if (hasReadyToPlayIntent(activeIntent, activeTableSeat) || hasWaitingIntent(activeIntent)) {
    return false
  }
  return Boolean(activeTableSeat?.tableId && activeTableSeat.status !== 'started')
}

/** Catalog play intent plus table seat for the intent banner. */
export async function fetchPlayerIntentState() {
  const [activeIntent, activeTableSeat] = await Promise.all([fetchMyActiveIntent(), fetchMyTableSeat()])
  return { activeIntent, activeTableSeat }
}
