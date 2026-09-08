import { useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { useActiveRoom } from '../rooms/ActiveRoomProvider'
import { navigateTo } from '../../lib/usePathname'
import { discardTable, leaveTable, sitAtTable, startTable } from '../../lib/tables'
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
import { Link } from '../ui/link'

/**
 * One table in a room the player never had to think about (JQ-132). Chat, the invite
 * code, multiple tables and the king role all still exist on the room underneath —
 * this view simply does not surface them.
 */
export default function GroupPage() {
  const { user } = useAuth()
  const { room, refresh } = useActiveRoom()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const table = selectGroupTable(room, user?.id)

  if (!table) {
    return (
      <main className="app-shell">
        <h1>Your Group</h1>
        <p className="status-message" role="status">
          This group has ended.
        </p>
        <Link className="self-start" href="/">
          Back to the catalog
        </Link>
      </main>
    )
  }

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
        <p className="status-message status-message-error" role="status">
          {error}
        </p>
      ) : null}

      <GroupInviteCard room={room} />

      <GroupSeatList
        table={table}
        userId={user?.id}
        busy={busy}
        onClaim={(seatKey) => run(() => sitAtTable(table.id, seatKey))}
        onLeaveSeat={() => run(() => leaveTable(table.id))}
      />

      <GroupSpectatorList players={playersPickingASeat(room, table)} userId={user?.id} />

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
    </main>
  )
}
