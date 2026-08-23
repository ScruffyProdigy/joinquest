import { useState } from 'react'
import { createRoom } from '../../lib/rooms'
import { useActiveRoom } from './ActiveRoomProvider'
import { Button } from '../ui/button'

export default function CreateRoomPanel() {
  const { openRoomFromCreate } = useActiveRoom()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function handleCreate() {
    setBusy(true)
    setError('')
    try {
      const room = await createRoom()
      openRoomFromCreate(room)
    } catch (err) {
      setError(err.message || 'Could not create room.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel-card" aria-label="Create a room">
      <h2>Room</h2>
      <p className="panel-copy">Start a private chat room and invite friends with a link or QR code.</p>
      {error ? <p className="status-message status-message-error">{error}</p> : null}
      <Button type="button" variant="default" onClick={handleCreate} disabled={busy}>
        {busy ? 'Creating…' : 'Create room'}
      </Button>
    </section>
  )
}
