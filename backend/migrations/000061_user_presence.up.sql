-- Presence is a property of the user, not of one of their queue rows.
--
-- Consumers ask "does this player hold any live socket at all", which no single
-- queue row can answer: a matched player may be holding a table page's socket
-- while their queue page's socket is long gone.
--
-- Postgres rather than Redis because the readers join this inside transactions
-- that already hold FOR UPDATE on game_queues (syncWaitingPartiesOnFormingTx,
-- ReconcileFormingModeQueue), and a Redis round-trip under those locks is worse
-- than a join.
CREATE TABLE user_presence (
    user_id          UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Sockets, not tabs. The frontend creates three independent graphql-ws
    -- clients (queue.js, rooms.js, tables.js), each with its own module-level
    -- client and no shared socket, so a single tab can hold three.
    --
    -- Every per-user subscription counts here, not just the queue one:
    -- QueueUpdated, MyTableSeatUpdated, TableUpdated, RoomUpdated,
    -- RoomMessageAdded, MatchResultUpdated. That is deliberate, not accidental
    -- breadth — the question this column answers is "does this user hold any
    -- live socket at all", so a player sitting in room chat is at their device
    -- and present. Pruning the room subscriptions would narrow the signal to
    -- "is this player watching the queue", which is a different question and the
    -- reason presence could not live on a queue row in the first place.
    connection_count INTEGER NOT NULL DEFAULT 0 CHECK (connection_count >= 0),
    -- Set exactly when connection_count reaches 0, cleared when it leaves 0.
    -- A raw timestamp rather than a staleness flag: each consumer applies its own
    -- window to it. This records "disconnected" (socket gone) and never "away"
    -- (socket alive, attention gone), which is a different signal owned elsewhere.
    disconnected_at  TIMESTAMP WITH TIME ZONE,
    updated_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_presence_disconnected
    ON user_presence (disconnected_at)
    WHERE disconnected_at IS NOT NULL;
