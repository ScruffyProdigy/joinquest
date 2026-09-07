import { useCallback, useEffect, useRef, useState } from 'react'
import SignInPanel from '../auth/SignInPanel'
import { useAuth } from '../auth/AuthProvider'
import { useActiveRoom } from './ActiveRoomProvider'
import { sendRoomMessage } from '../../lib/rooms'
import { discardTable, displayName, leaveTable, sitAtTable, startTable, startTableBackfill } from '../../lib/tables'
import { accentColorFor } from '../../lib/gameAccent'
import { IconClose } from '../icons/ShareIcons'
import RoomShareToolbar from './RoomShareToolbar'
import TableCard from './TableCard'
import { useActiveTableSeat } from '../games/useActiveTableSeat'
import { useActiveIntent } from '../games/useActiveIntent'
import PlayerAvatar from '../avatars/PlayerAvatar'
import { useIdentityPrompt } from '../avatars/IdentityPromptProvider'
import { Card, CardContent } from '../ui/card'
import { Button } from '../ui/button'
import { cn } from '../../lib/utils'

function mergeMessage(messages, incoming) {
  if (!incoming?.id) {
    return messages
  }
  if (messages.some((msg) => msg.id === incoming.id)) {
    return messages
  }
  return [...messages, incoming]
}

export default function RoomPanel({ compact = false }) {
  const { user, loading: authLoading } = useAuth()
  const {
    room,
    messages,
    setMessages,
    loading,
    error,
    setError,
    handleLeave,
    mergeTableUpdate,
    refresh,
    unreadCount,
    markRead,
  } = useActiveRoom()
  const { refresh: refreshTableSeat } = useActiveTableSeat()
  const { refresh: refreshIntent } = useActiveIntent()
  const { requireIdentity } = useIdentityPrompt()
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [tableBusy, setTableBusy] = useState(false)
  const chatLogRef = useRef(null)
  const chatAtBottomRef = useRef(true)
  const [chatAtBottom, setChatAtBottom] = useState(true)

  const syncChatScrollState = useCallback(() => {
    const log = chatLogRef.current
    if (!log) {
      return
    }
    const atBottom = log.scrollHeight - log.scrollTop - log.clientHeight <= 24
    chatAtBottomRef.current = atBottom
    setChatAtBottom(atBottom)
    if (atBottom) {
      markRead()
    }
  }, [markRead])

  const scrollChatToBottom = useCallback((behavior = 'smooth') => {
    const log = chatLogRef.current
    if (!log) {
      return
    }
    log.scrollTo({ top: log.scrollHeight, behavior })
    chatAtBottomRef.current = true
    setChatAtBottom(true)
    markRead()
  }, [markRead])

  useEffect(() => {
    chatAtBottomRef.current = true
    setChatAtBottom(true)
  }, [room?.id])

  useEffect(() => {
    if (!messages.length) {
      return
    }
    const log = chatLogRef.current
    if (!log) {
      return
    }
    if (chatAtBottomRef.current) {
      log.scrollTo({ top: log.scrollHeight, behavior: 'smooth' })
      markRead()
    }
  }, [messages, markRead])

  async function handleSend(event) {
    event.preventDefault()
    const body = draft.trim()
    if (!body || !room?.id || busy) {
      return
    }
    setBusy(true)
    setError('')
    try {
      const msg = await sendRoomMessage(room.id, body)
      setMessages((prev) => mergeMessage(prev, msg))
      setDraft('')
      requestAnimationFrame(() => scrollChatToBottom())
    } catch (err) {
      setError(err.message || 'Could not send message.')
    } finally {
      setBusy(false)
    }
  }

  async function runTableAction(action) {
    setTableBusy(true)
    setError('')
    try {
      await action()
      await refresh()
      await refreshTableSeat()
    } catch (err) {
      setError(err.message || 'Table action failed.')
    } finally {
      setTableBusy(false)
    }
  }

  async function handleSit(tableId, seatKey) {
    setTableBusy(true)
    setError('')
    try {
      // Claiming a seat is the intent, so this is where a nameless player is
      // asked. The table and seat ride along in the closure and the sit runs
      // itself once the profile saves.
      await requireIdentity(async () => {
        const updated = await sitAtTable(tableId, seatKey)
        mergeTableUpdate(updated)
        await refreshTableSeat()
        void refresh()
      })
    } catch (err) {
      setError(err.message || 'Could not sit at that seat.')
    } finally {
      setTableBusy(false)
    }
  }

  async function handleLeaveTable(tableId) {
    await runTableAction(async () => {
      await leaveTable(tableId)
    })
  }

  async function handleStartTable(tableId) {
    await runTableAction(async () => {
      const result = await startTable(tableId)
      if (result?.joinUrl) {
        window.location.assign(result.joinUrl)
      }
    })
  }

  async function handleLookForGroup(tableId, queueId) {
    await runTableAction(async () => {
      const result = await startTableBackfill(tableId, queueId)
      await refreshIntent()
      if (result?.joinUrl) {
        window.location.assign(result.joinUrl)
      }
    })
  }

  async function handleDiscardTable(tableId) {
    await runTableAction(async () => {
      await discardTable(tableId)
    })
  }

  if (authLoading) {
    return <p className="status-message">Loading session…</p>
  }

  if (!user) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <h2 className="font-heading text-xl text-foreground">Join room</h2>
        <p className="panel-copy">Sign in to enter the chat.</p>
        <SignInPanel />
      </div>
    )
  }

  if (!room && loading) {
    return <p className="status-message">{busy ? 'Joining room…' : 'Loading room…'}</p>
  }

  if (!room) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <p className="status-message status-message-error">{error || 'Could not load room.'}</p>
      </div>
    )
  }

  const memberCount = room.members?.length ?? 0
  const tables = room.tables ?? []
  const accent = accentColorFor(
    tables[0]?.game?.slug || tables[0]?.game?.id || 'room',
    tables[0]?.game?.accentColor,
  )

  const membersList = (
    <ul className="flex flex-col gap-2">
      {room.members.map((member) => (
        <li key={member.id} className="flex items-center gap-2 text-sm text-foreground">
          <PlayerAvatar user={member} size="sm" />
          <span>
            {displayName(member)}
            {member.id === room.host?.id ? ' (host)' : ''}
          </span>
        </li>
      ))}
    </ul>
  )

  const tablesList = (
    <ul className="flex flex-col gap-4">
      {tables.map((table) => (
        <li key={table.id}>
          <TableCard
            table={table}
            busy={tableBusy}
            onSit={(seatKey) => handleSit(table.id, seatKey)}
            onLeave={() => handleLeaveTable(table.id)}
            onStart={() => handleStartTable(table.id)}
            onLookForGroup={(queueId) => handleLookForGroup(table.id, queueId)}
            onDiscard={() => handleDiscardTable(table.id)}
          />
        </li>
      ))}
    </ul>
  )

  return (
    <div className={cn('flex h-full flex-col', compact && 'text-sm')}>
      <div className="flex-1 overflow-y-auto">
        <header
          className="flex items-start justify-between gap-3 rounded-b-2xl px-6 py-8 text-foreground"
          style={{ background: accent.headerBg }}
        >
          <div>
            <h2 className="font-heading text-2xl font-bold">Room {room.inviteCode}</h2>
            <p className="text-sm opacity-90">
              {memberCount} {memberCount === 1 ? 'member' : 'members'}
            </p>
          </div>
        </header>

        <div className="flex flex-col gap-4 p-4">
          {error ? <p className="status-message status-message-error">{error}</p> : null}

          <details className="rounded-xl border border-border bg-card">
            <summary className="cursor-pointer px-4 py-3 text-sm font-semibold text-foreground">
              Members ({memberCount})
            </summary>
            <div className="px-4 pb-4">{membersList}</div>
          </details>

          <RoomShareToolbar joinUrl={room.joinUrl} inviteCode={room.inviteCode} />

          {tables.length > 0 ? (
            <details className="rounded-xl border border-border bg-card" open>
              <summary className="cursor-pointer px-4 py-3 text-sm font-semibold text-foreground">
                Tables ({tables.length})
              </summary>
              <div className="flex flex-col gap-4 px-4 pb-4">{tablesList}</div>
            </details>
          ) : null}

          <Card className="flex flex-1 flex-col gap-3 py-4">
            <CardContent className="flex flex-1 flex-col gap-3 px-4">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-semibold text-foreground">Chat</h3>
                {unreadCount > 0 && !chatAtBottom ? (
                  <button
                    type="button"
                    className="rounded-full bg-primary px-3 py-1 text-xs font-semibold text-primary-foreground"
                    onClick={() => scrollChatToBottom()}
                    aria-label={`${unreadCount} unread messages — jump to latest`}
                  >
                    {unreadCount > 99 ? '99+' : unreadCount} new
                  </button>
                ) : null}
              </div>
              <div
                ref={chatLogRef}
                className="flex max-h-64 flex-col gap-2 overflow-y-auto"
                aria-live="polite"
                onScroll={syncChatScrollState}
              >
                {messages.length === 0 ? (
                  <p className="panel-copy">Say hello — messages appear here for everyone in the room.</p>
                ) : (
                  messages.map((msg) => (
                    <article key={msg.id} className="rounded-lg bg-secondary/60 px-3 py-2">
                      <p className="text-xs font-semibold text-muted-foreground">{displayName(msg.author)}</p>
                      <p className="text-sm text-foreground">{msg.body}</p>
                    </article>
                  ))
                )}
              </div>
              <form className="flex items-center gap-2" onSubmit={handleSend}>
                <label htmlFor="room-message" className="sr-only">
                  Message
                </label>
                <input
                  id="room-message"
                  type="text"
                  className="h-9 flex-1 rounded-md border border-border bg-input-background px-3 text-sm text-foreground outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  value={draft}
                  onChange={(event) => setDraft(event.target.value)}
                  maxLength={2000}
                  disabled={busy}
                  autoComplete="off"
                  placeholder="Message"
                />
                <Button type="submit" size="sm" disabled={busy || !draft.trim()}>
                  Send
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>
      </div>

      <div className="border-t border-border p-4">
        <Button type="button" variant="secondary" className="w-full" onClick={handleLeave} disabled={busy || loading}>
          <IconClose aria-hidden="true" />
          Leave room
          <IconClose aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}
