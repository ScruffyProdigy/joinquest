-- JQ-143: an append-only record of what players did, captured broadly now so that
-- difficulty -- especially a game's floor, how hard it is to get into -- can be
-- correlated against it later.
--
-- Nobody yet knows which measurable signals track the human experience of a game
-- being hard to learn. That is deliberately a separate analysis ticket. This table
-- exists so that when someone goes looking, the history is already there; a signal
-- nobody recorded cannot be correlated after the fact.
--
-- The shape is an envelope, not a wide table of guessed columns: who, which game,
-- which mode, which session, when, and a JSON payload for whatever that particular
-- event type happens to know. Adding a signal later is an INSERT with a new
-- event_type, not a migration. That is the whole reason "record everything" is safe
-- here rather than a liability -- no schema decision has to be right in advance.
CREATE TABLE IF NOT EXISTS player_activity_events (
    id BIGSERIAL PRIMARY KEY,

    -- Free text, deliberately not an enum. A CHECK constraint or a Postgres enum
    -- would make every new signal a migration, which is exactly what this table is
    -- designed to avoid. The known types are constants in internal/activity, where
    -- adding one costs a line of Go and nothing in the database.
    event_type TEXT NOT NULL,

    -- Who emitted the row. 'lobby' is the only writer today. Games knowing things
    -- the lobby never will -- that a player fumbled a tutorial, say -- is a real
    -- possibility the ticket calls out and defers to a later protocol change, so the
    -- envelope carries the column now rather than forcing an ALTER TABLE onto
    -- whoever picks that up. Cheap here, awkward to retrofit.
    source TEXT NOT NULL DEFAULT 'lobby',

    -- The lobby user id, and per the ticket's retention rule the only identifying
    -- thing permitted anywhere in this row. Nullable because some events are real
    -- and player-less (a queue that never formed a match).
    --
    -- No foreign key, here or below. That is load-bearing, not an oversight: a
    -- foreign key makes an instrumentation INSERT fail when the referent is gone,
    -- and makes deleting a user wait on this table. Instrumentation that can fail a
    -- player-facing write, or block one, is the failure mode this ticket explicitly
    -- forbids. Orphan rows are handled by the retention sweep instead.
    user_id UUID,
    game_id UUID,

    -- Text, like rating_match_inputs, not a games_modes.id. Mode rows come and go
    -- with manifest sync and the natural key is what survives a re-register. History
    -- has to outlive the row that described it.
    mode_key TEXT,
    session_id UUID,

    -- When the thing being described happened. Supplied by the caller, because for
    -- several events the honest answer already exists on a database clock:
    -- game_sessions.started_at, for instance, is the instant the matchmaking
    -- transaction began. Emitting after commit and stamping NOW() here would quietly
    -- substitute this process's clock, drifting by however long the rest of that
    -- transaction took.
    --
    -- Which clock produced it therefore varies by event type: match_started carries
    -- the database's, everything else the emitting process's. Subtracting occurred_at
    -- ACROSS those two groups yields a duration plus an unknown skew, not a duration.
    -- See docs/player-activity-events.md.
    occurred_at TIMESTAMPTZ NOT NULL,

    -- When the row was actually written, always on the DATABASE's clock -- the writer
    -- deliberately never supplies this, so the default below is what stamps it.
    --
    -- Kept separate from occurred_at rather than collapsed into it: writes are
    -- asynchronous by design, so the gap between the two is roughly the
    -- instrumentation's own lag. Roughly, and not exactly, because occurred_at may
    -- have come from the emitting process's clock rather than this one, and the two
    -- disagree -- by seconds, in both directions, on the local Docker stack. A
    -- recorded_at that appeared to precede its own event would be nonsense on an
    -- append-only table, which is why this one column is pinned to the database.
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Whatever this event type knows. Per the ticket: no personally identifying data
    -- beyond user_id above. Ids, enums, counts and durations only -- never an email,
    -- display name, IP or user agent.
    payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

-- Three indexes, not more. Every index is a write cost paid on an append-only table
-- on the hot matchmaking path, so each one below has a query that needs it and
-- nothing is added speculatively -- notably no GIN on payload, which has no reader
-- yet. The ticket's own guidance is to stay on Postgres and not add infrastructure
-- ahead of the volume that justifies it; the same restraint applies inside the table.

-- Per-player longitudinal work: a player's history in order, which is what "did they
-- come back, and how long did it take" is asking for.
CREATE INDEX IF NOT EXISTS idx_player_activity_events_user_occurred
    ON player_activity_events (user_id, occurred_at)
    WHERE user_id IS NOT NULL;

-- The per-game playtest summary, which slices one game's events by type over a
-- window. event_type sits in the middle because that summary counts each type
-- separately rather than scanning the game's whole history once.
CREATE INDEX IF NOT EXISTS idx_player_activity_events_game_type_occurred
    ON player_activity_events (game_id, event_type, occurred_at)
    WHERE game_id IS NOT NULL;

-- The retention sweep, whose only predicate is an age. Not served by either index
-- above: both are partial, so neither covers rows with a NULL user_id or game_id,
-- and the sweep has to be able to reach every row.
CREATE INDEX IF NOT EXISTS idx_player_activity_events_occurred
    ON player_activity_events (occurred_at);
