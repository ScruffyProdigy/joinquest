import RoomShareToolbar from '../rooms/RoomShareToolbar'

/**
 * The group view shares by QR and link. The invite code still exists on the room and
 * still works everywhere else — it is simply not the thing a player needs here.
 */
export default function GroupInviteCard({ room }) {
  return (
    <section className="px-4 pt-4" aria-label="Invite friends">
      <RoomShareToolbar joinUrl={room?.joinUrl} inviteCode={room?.inviteCode} showCode={false} />
    </section>
  )
}
