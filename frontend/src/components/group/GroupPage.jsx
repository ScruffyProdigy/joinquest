import { useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { useActiveRoom } from '../rooms/ActiveRoomProvider'
import { navigateTo } from '../../lib/usePathname'
import { discardTable, leaveTable, sitAtTable, startTable } from '../../lib/tables'
import { fetchModeQueueOptions } from '../../lib/games'
import { GROUP_FIND_SOMETHING_NEW, GROUP_TAKE_SEAT, GROUP_TAKING_SEAT } from '../../lib/playerCopy'
import { hasReadyToPlayIntent } from '../../lib/intent'
import {
  groupCtaState,
  isLastSeatedPlayer,
  playersPickingASeat,
  selectGroupTable,
} from '../../lib/group'
import GroupHeader from './GroupHeader'
import GroupInviteCard from './GroupInviteCard'
import GroupSeatList from './GroupSeatList'
import GroupSpectatorList from './GroupSpectatorList'
import GroupStartBar from './GroupStartBar'
import LaunchStep from '../games/LaunchStep'
import PreQueueOptionsSheet from '../games/PreQueueOptionsSheet'
import { Link } from '../ui/link'

/**
 * One table in a room the player never had to think about (JQ-132). Chat, the invite
 * code, multiple tables and the king role all still exist on the room underneath —
 * this view simply does not surface them.
 */
export default function GroupPage({ intent }) {
  const { user } = useAuth()
  const { room, refresh } = useActiveRoom()
  const {
    activeIntent = null,
    activeTableSeat = null,
    loading: intentLoading = false,
    busy: intentBusy = false,
    leaveError = null,
    handleLeave: leaveActiveGame,
  } = intent ?? {}
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  // The seat the player has asked for but not yet paid the picker for. Holding it here is
  // what lets the sheet's confirm finish a claim the click only started.
  const [pendingSeatKey, setPendingSeatKey] = useState(null)
  const [queueOptions, setQueueOptions] = useState(null)

  const table = selectGroupTable(room, user?.id)

  /*
    Starting the game is what this page is for, so the launch moment JQ-136 built for
    the queue belongs here too. Everyone reaches it off their own `myTableSeat`, which
    the server pushes a signed launch URL to for every seated player — the Start
    button's result only ever existed for whoever pressed it. This is checked before
    the table, because the per-user seat event and the room-wide table event race:
    the seat saying "started" is the answer either way.
  */
  if (hasReadyToPlayIntent(activeIntent, activeTableSeat)) {
    return (
      <main className="app-shell waiting-page">
        <LaunchStep
          activeIntent={activeIntent}
          activeTableSeat={activeTableSeat}
          busy={intentBusy}
          leaveError={leaveError}
          onLeave={leaveActiveGame}
        />
      </main>
    )
  }

  if (!table) {
    return (
      <main className="app-shell px-6 py-8">
        <h1>Your Group</h1>
        {/*
          A started table leaves `room.tables` at once — the server only returns forming
          ones — so its absence is not by itself evidence the group ended. Until the
          seat lookup that the same update kicks off has answered, the page says only
          that it is still looking.
        */}
        <p className="status-message" role="status">
          {intentLoading ? 'Checking your group…' : 'This group has ended.'}
        </p>
        <Link className="self-start" href="/">
          Back to the catalog
        </Link>
      </main>
    )
  }

  const preQueueGroups = table.mode?.preQueueGroups ?? []

  async function run(action) {
    setBusy(true)
    setError('')
    try {
      await action()
      await refresh?.()
    } catch (err) {
      setError(err.message || 'Something went wrong.')
    } finally {
      setBusy(false)
    }
  }

  /**
   * A mode with option groups routes every seat claim through the picker first: each player
   * answers it for themselves as they sit down, because nobody may choose another player's
   * champion, kit or deck. Nothing is pre-selected, including for a group just back from a
   * round — mixing up who brings what is most of the point (JQ-232).
   */
  async function handleClaim(seatKey) {
    if (preQueueGroups.length === 0) {
      await run(() => sitAtTable(table.id, seatKey))
      return
    }
    setError('')
    setQueueOptions(null)
    setPendingSeatKey(seatKey)
    try {
      const options = await fetchModeQueueOptions(table.game?.id, table.mode?.id, user?.id)
      setQueueOptions(options)
    } catch (err) {
      // The roster comes only from the game, so failing to reach it is the sheet's own
      // "unavailable" state rather than an error on the page behind it: there is no safe
      // fallback that would let this player sit down anyway.
      setQueueOptions({ available: false, unavailableReason: err.message || '', groups: [] })
    }
  }

  function closePicker() {
    setPendingSeatKey(null)
    setQueueOptions(null)
  }

  async function handleOptionsConfirmed(selections) {
    const seatKey = pendingSeatKey
    closePicker()
    await run(() => sitAtTable(table.id, seatKey, selections))
  }

  /**
   * Presence on this page is the seat. Deliberate navigation away releases it;
   * a reload, a closed tab or a dropped connection deliberately do not, so there is
   * no unload handler here on purpose.
   */
  async function handleLeaveGroup() {
    setBusy(true)
    setError('')
    const lastOut = isLastSeatedPlayer(table, user?.id)
    try {
      await leaveTable(table.id)
      if (lastOut) {
        // leaveTable only deletes the seat; without this the emptied table lingers
        // in the player's room forever.
        await discardTable(table.id).catch(() => {})
      }
      await refresh?.()
    } catch {
      // leaving must never trap the player on this page
    } finally {
      setBusy(false)
      navigateTo('/', { replace: true })
    }
  }

  const cta = groupCtaState(table, user?.id)

  return (
    <main className="app-shell app-shell--group">
      <GroupHeader table={table} room={room} busy={busy} onLeave={handleLeaveGroup} />

      {error ? (
        <p className="status-message status-message-error px-4 pt-4" role="status">
          {error}
        </p>
      ) : null}

      <GroupInviteCard room={room} game={table.game} />

      <GroupSeatList
        table={table}
        userId={user?.id}
        busy={busy}
        onClaim={handleClaim}
        onLeaveSeat={() => run(() => leaveTable(table.id))}
      />

      <GroupSpectatorList players={playersPickingASeat(room, table, user?.id)} userId={user?.id} />

      {/*
        A rejoining group lands here, and for some of them the answer is "not this again".
        The header's back arrow already leaves, but it reads as undo rather than as a
        choice, so the way back to matchmaking is named (JQ-232).
      */}
      <p className="px-4 pb-2">
        <Link href="/">{GROUP_FIND_SOMETHING_NEW}</Link>
      </p>

      <GroupStartBar
        cta={cta}
        busy={busy}
        onStart={() =>
          run(async () => {
            const result = await startTable(table.id)
            if (result?.joinUrl) {
              window.location.href = result.joinUrl
            }
          })
        }
      />

      <PreQueueOptionsSheet
        open={pendingSeatKey !== null}
        gameName={table.game?.name}
        modeName={table.mode?.displayName}
        groups={preQueueGroups}
        queueOptions={queueOptions}
        busy={busy}
        confirmLabel={GROUP_TAKE_SEAT}
        busyLabel={GROUP_TAKING_SEAT}
        confirmIcon={null}
        onConfirm={handleOptionsConfirmed}
        onClose={closePicker}
      />
    </main>
  )
}
