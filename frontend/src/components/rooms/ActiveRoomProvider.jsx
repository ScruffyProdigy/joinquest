import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { navigateTo } from '../../lib/usePathname'
import {
  fetchMyRoom,
  fetchRoom,
  joinRoom,
  leaveRoom as leaveRoomApi,
  subscribeToRoom,
} from '../../lib/rooms'
import { prefetchSubscriptionAuth } from '../../lib/queue'
import { readRoomInviteHint, readRoomMemberHint, writeRoomDockHint } from '../../lib/roomDockHint'
import { groupLandingPath } from '../../lib/group'
import { onTabVisible } from '../../lib/tabVisibility'
import { fetchMyTableSeat, mergeTableRecord, TABLE_UPDATED_EVENT, tableShouldLeaveRoomList } from '../../lib/tables'

const ActiveRoomContext = createContext(null)

function mergeMessage(messages, incoming) {
  if (!incoming?.id) {
    return messages
  }
  if (messages.some((msg) => msg.id === incoming.id)) {
    return messages
  }
  return [...messages, incoming]
}

function countUnread(messages, lastReadId, userId) {
  if (!messages?.length) {
    return 0
  }
  if (!lastReadId) {
    return messages.filter((msg) => msg.author?.id !== userId).length
  }
  let pastLastRead = false
  let count = 0
  for (const msg of messages) {
    if (msg.id === lastReadId) {
      pastLastRead = true
      continue
    }
    if (pastLastRead && msg.author?.id !== userId) {
      count += 1
    }
  }
  return count
}

export function ActiveRoomProvider({ children, pendingInviteCode = null }) {
  const { user, loading: authLoading } = useAuth()
  const [room, setRoom] = useState(null)
  const [messages, setMessages] = useState([])
  const [roomOpen, setRoomOpen] = useState(() => Boolean(pendingInviteCode))
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [lastReadMessageId, setLastReadMessageId] = useState(null)
  const [memberHint, setMemberHint] = useState(() => readRoomMemberHint())
  const [tableSeatInviteCode, setTableSeatInviteCode] = useState('')
  // Table ids from the snapshot last applied, so a refresh can tell that one went away.
  const knownTableIdsRef = useRef([])
  const unsubscribeRef = useRef(null)
  // Callers legitimately hold on to callbacks across a sign-in, a guest session
  // being created, or a sign-out — `requireIdentity` replays the closure it was
  // handed (JQ-84). Reading the session through a ref means those callbacks act
  // on the session the player has now, not the one their render captured.
  const userRef = useRef(user)
  userRef.current = user

  const hasRoomMembership = useMemo(
    () => Boolean(room?.inviteCode || memberHint || tableSeatInviteCode),
    [room?.inviteCode, memberHint, tableSeatInviteCode],
  )

  const unreadCount = useMemo(
    () => countUnread(messages, lastReadMessageId, user?.id),
    [messages, lastReadMessageId, user?.id],
  )

  const markRead = useCallback(() => {
    const latest = messages[messages.length - 1]
    if (latest?.id) {
      setLastReadMessageId(latest.id)
    }
  }, [messages])

  const applyRoom = useCallback((nextRoom, nextMessages = []) => {
    // A refreshed snapshot that has dropped a table is the same news the subscription
    // delivers below: a table stopped forming. Announce it either way, so a player
    // whose group started between polls re-checks their own seat now rather than
    // waiting on the next one — the seat is what carries them into the game.
    const nextTableIds = (nextRoom?.tables ?? []).map((table) => table.id)
    const droppedTable = knownTableIdsRef.current.some((id) => !nextTableIds.includes(id))
    knownTableIdsRef.current = nextTableIds

    setRoom((prev) => {
      if (prev?.id !== nextRoom?.id) {
        const latest = nextMessages[nextMessages.length - 1]
        setLastReadMessageId(latest?.id ?? null)
      }
      return nextRoom
    })
    setMessages(nextMessages)
    setError('')
    writeRoomDockHint({ member: true, inviteCode: nextRoom?.inviteCode })
    setMemberHint(true)
    if (droppedTable) {
      window.dispatchEvent(new CustomEvent(TABLE_UPDATED_EVENT))
    }
  }, [])

  const mergeTableUpdate = useCallback((updatedTable) => {
    if (!updatedTable?.id) {
      return
    }
    if (tableShouldLeaveRoomList(updatedTable)) {
      setRoom((prev) => {
        if (!prev) {
          return prev
        }
        return {
          ...prev,
          tables: (prev.tables ?? []).filter((table) => table.id !== updatedTable.id),
        }
      })
      window.dispatchEvent(new CustomEvent(TABLE_UPDATED_EVENT))
      return
    }
    setRoom((prev) => {
      if (!prev) {
        return prev
      }
      const tables = prev.tables ?? []
      const idx = tables.findIndex((t) => t.id === updatedTable.id)
      if (idx === -1) {
        return { ...prev, tables: [...tables, updatedTable] }
      }
      const next = [...tables]
      next[idx] = mergeTableRecord(tables[idx], updatedTable)
      return { ...prev, tables: next }
    })
    window.dispatchEvent(new CustomEvent(TABLE_UPDATED_EVENT))
  }, [])

  const loadRoomSnapshot = useCallback(async () => {
    try {
      const mine = await fetchMyRoom()
      if (mine) {
        applyRoom(mine, mine.messages || [])
        return mine
      }
    } catch {
      // try fallbacks below
    }

    const hintCode = readRoomInviteHint()
    if (hintCode) {
      try {
        const fromHint = await fetchRoom(hintCode)
        if (fromHint) {
          applyRoom(fromHint, fromHint.messages || [])
          return fromHint
        }
      } catch {
        // try fallbacks below
      }
    }

    try {
      const seat = await fetchMyTableSeat()
      const code = seat?.inviteCode?.trim().toUpperCase() || ''
      if (code) {
        writeRoomDockHint({ member: true, inviteCode: code })
        setMemberHint(true)
        setTableSeatInviteCode(code)
        const fromSeat = await fetchRoom(code)
        if (fromSeat) {
          applyRoom(fromSeat, fromSeat.messages || [])
          return fromSeat
        }
      } else {
        setTableSeatInviteCode('')
      }
    } catch {
      // all fallbacks exhausted
    }

    return null
  }, [applyRoom])

  const refresh = useCallback(async () => {
    // Signed out, `refresh` is a no-op rather than a reset: clearing the room is
    // owned by the sign-out effect below, which fires when the session actually
    // changes. A caller holding a stale `refresh` must never be able to wipe the
    // room that a newer session just loaded.
    if (!userRef.current) {
      return null
    }
    setLoading(true)
    try {
      const loaded = await loadRoomSnapshot()
      if (loaded) {
        return loaded
      }
      setRoom(null)
      setMessages([])
      if (!readRoomMemberHint()) {
        writeRoomDockHint({ member: false })
        setMemberHint(false)
      }
      return null
    } catch {
      return null
    } finally {
      setLoading(false)
    }
  }, [loadRoomSnapshot])

  useEffect(() => {
    if (authLoading || !user) {
      return
    }
    void fetchMyTableSeat()
      .then((seat) => {
        const code = seat?.inviteCode?.trim().toUpperCase() || ''
        setTableSeatInviteCode(code)
        if (code) {
          writeRoomDockHint({ member: true, inviteCode: code })
          setMemberHint(true)
        }
      })
      .catch(() => {})
  }, [authLoading, user])

  const openRoomByCode = useCallback(
    async (inviteCode, { openPanel = true } = {}) => {
      if (!user || !inviteCode) {
        return null
      }
      setLoading(true)
      setError('')
      try {
        await joinRoom(inviteCode)
        const full = await fetchRoom(inviteCode)
        if (!full) {
          setError('Room not found.')
          return null
        }
        applyRoom(full, full.messages || [])
        if (openPanel) {
          setRoomOpen(true)
          // A share link to a room with a single forming table is a group invite:
          // land on the group, not the room panel (JQ-132). Everything else keeps
          // the room surfaces exactly as they were.
          navigateTo(groupLandingPath(full) ?? `/room/${full.inviteCode}`, { replace: true })
        }
        return full
      } catch (err) {
        setError(err.message || 'Could not join room.')
        return null
      } finally {
        setLoading(false)
      }
    },
    [user, applyRoom],
  )

  const openRoom = useCallback(async () => {
    setRoomOpen(true)
    setError('')

    if (room?.inviteCode) {
      navigateTo(`/room/${room.inviteCode}`)
      return
    }

    setLoading(true)
    try {
      const target = await loadRoomSnapshot()
      if (!target?.inviteCode) {
        setError('Could not load your room. Try again in a moment.')
        return
      }
      navigateTo(`/room/${target.inviteCode}`)
    } catch (err) {
      setError(err.message || 'Could not load your room.')
    } finally {
      setLoading(false)
    }
  }, [room, loadRoomSnapshot])

  const dismissRoom = useCallback(() => {
    setRoomOpen(false)
    navigateTo('/', { replace: true })
  }, [])

  const openRoomFromCreate = useCallback(
    (createdRoom) => {
      applyRoom(createdRoom, [])
      setRoomOpen(true)
      navigateTo(`/room/${createdRoom.inviteCode}`)
    },
    [applyRoom],
  )

  const handleLeave = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      await leaveRoomApi()
      setRoom(null)
      setMessages([])
      setRoomOpen(false)
      setLastReadMessageId(null)
      writeRoomDockHint({ member: false })
      setMemberHint(false)
      setTableSeatInviteCode('')
      navigateTo('/', { replace: true })
    } catch (err) {
      setError(err.message || 'Could not leave room.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (authLoading) {
      return
    }
    if (!user) {
      setRoom(null)
      setMessages([])
      setRoomOpen(false)
      writeRoomDockHint({ member: false })
      setMemberHint(false)
      setTableSeatInviteCode('')
      return
    }
    if (pendingInviteCode) {
      void openRoomByCode(pendingInviteCode, { openPanel: true })
      return
    }
    void refresh()
  }, [authLoading, user, pendingInviteCode, openRoomByCode, refresh])

  useEffect(() => {
    if (authLoading || !user || room?.inviteCode || pendingInviteCode) {
      return
    }
    if (!tableSeatInviteCode && !readRoomInviteHint()) {
      return
    }
    void refresh()
  }, [authLoading, user, room?.inviteCode, tableSeatInviteCode, pendingInviteCode, refresh])

  useEffect(() => {
    if (authLoading || !user) {
      return undefined
    }
    return onTabVisible(() => {
      void refresh()
    })
  }, [authLoading, user, refresh])

  useEffect(() => {
    if (!user || !room?.id) {
      return undefined
    }
    const hasFormingTables = (room.tables ?? []).some((table) => !tableShouldLeaveRoomList(table))
    if (!hasFormingTables) {
      return undefined
    }
    const timer = window.setInterval(() => {
      void refresh()
    }, 3000)
    return () => window.clearInterval(timer)
  }, [user, room?.id, room?.tables, refresh])

  useEffect(() => {
    unsubscribeRef.current?.()
    unsubscribeRef.current = null

    if (!user || !room?.id) {
      return undefined
    }

    let cancelled = false

    void (async () => {
      try {
        await prefetchSubscriptionAuth()
        if (cancelled) {
          return
        }
        const unsubscribe = await subscribeToRoom(room.id, {
          onRoomUpdate: (updated) => {
            setRoom((prev) => (prev ? { ...prev, ...updated } : prev))
          },
          onMessage: (msg) => {
            setMessages((prev) => mergeMessage(prev, msg))
          },
          onTableUpdate: (table) => {
            mergeTableUpdate(table)
          },
        })
        if (cancelled) {
          unsubscribe()
          return
        }
        unsubscribeRef.current = unsubscribe
      } catch {
        // chat still works; refresh on focus
      }
    })()

    return () => {
      cancelled = true
      unsubscribeRef.current?.()
      unsubscribeRef.current = null
    }
  }, [user, room?.id, mergeTableUpdate])

  const value = useMemo(
    () => ({
      room,
      messages,
      setMessages,
      roomOpen,
      loading,
      error,
      setError,
      unreadCount,
      refresh,
      openRoom,
      dismissRoom,
      openRoomByCode,
      openRoomFromCreate,
      handleLeave,
      markRead,
      hasRoomMembership,
      mergeTableUpdate,
    }),
    [
      room,
      messages,
      roomOpen,
      loading,
      error,
      unreadCount,
      hasRoomMembership,
      refresh,
      openRoom,
      dismissRoom,
      openRoomByCode,
      openRoomFromCreate,
      handleLeave,
      markRead,
      mergeTableUpdate,
    ],
  )

  return <ActiveRoomContext.Provider value={value}>{children}</ActiveRoomContext.Provider>
}

// The accessor hook lives with its provider, the way useAuth and
// useIdentityPrompt do. Splitting it out to satisfy fast refresh would move
// every consumer's import for no gain at runtime.
// eslint-disable-next-line react-refresh/only-export-components
export function useActiveRoom() {
  const context = useContext(ActiveRoomContext)
  if (!context) {
    throw new Error('useActiveRoom must be used within ActiveRoomProvider')
  }
  return context
}
