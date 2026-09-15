-- JQ-143: what survives the retention window.
--
-- Raw events are kept for 180 days and then deleted. The ticket is blunt about the
-- risk in that: longitudinal analysis will be wanted at a point when the volume does
-- not yet justify keeping everything, and deleted is deleted. These two tables are
-- the compromise it suggests -- aggregates that outlive the raw rows they were
-- computed from, so that the questions worth asking in two years are still
-- answerable from data collected today.
--
-- What is deliberately given up: the ability to ask a NEW question of raw events
-- older than 180 days. That is a real loss and it is the accepted one. Keeping every
-- row forever to preserve it would mean an unbounded append-only table on a platform
-- that has not yet measured its own volume, which the ticket rules out.

-- The funnel, per game and mode and day. Enough to answer "is this game getting
-- easier to get into over time" long after the events behind it are gone.
CREATE TABLE IF NOT EXISTS player_activity_daily (
    day         DATE NOT NULL,
    game_id     UUID NOT NULL,
    -- Empty string, not NULL, for events with no mode: this is a primary key column,
    -- and NULLs in a primary key would make the upsert below silently insert a new
    -- row every sweep instead of updating the existing one.
    mode_key    TEXT NOT NULL DEFAULT '',
    event_type  TEXT NOT NULL,
    event_count BIGINT NOT NULL,
    -- Distinct players on that day. Not summable across days -- a player active on
    -- both Monday and Tuesday counts once in each -- and any reader adding these up
    -- to get a weekly figure will overcount. Stored anyway because the daily number
    -- is the one worth having, and the alternative is not storing it at all.
    distinct_users BIGINT NOT NULL,
    PRIMARY KEY (day, game_id, mode_key, event_type)
);

-- The floor signal itself, preserved per player and game rather than aggregated away.
--
-- This is the row the whole ticket is pointed at: someone's first-ever match of a
-- game, how it ended, and whether they ever came back. Aggregating it into a daily
-- count would destroy exactly the correlation a later analysis needs, so it is kept
-- at full resolution -- one row per player per game, which grows with players rather
-- than with play, and is therefore affordable to keep indefinitely.
--
-- It carries no personal data beyond the lobby user id, the same rule the raw events
-- follow.
CREATE TABLE IF NOT EXISTS player_first_match_summary (
    user_id UUID NOT NULL,
    game_id UUID NOT NULL,

    first_session_id UUID,
    first_started_at TIMESTAMPTZ NOT NULL,

    -- False means the game never reported how this match ended. Kept as its own
    -- column rather than inferred from a NULL reason, because "we did not hear" and
    -- "it ended without a reason" are different facts and only one of them is about
    -- the game being hard.
    first_outcome_observed BOOLEAN NOT NULL DEFAULT FALSE,
    first_finish_reason    TEXT,

    -- NULL means no second match had been seen when this row was last refreshed. For
    -- a player whose first match is recent that means "not yet", not "never" --
    -- refreshed_at is what tells those apart.
    second_started_at TIMESTAMPTZ,

    refreshed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, game_id)
);

-- A merged-away guest's summary rows move with the rest of their history
-- (see carrySourceHistoryTx), so this index is what makes that update cheap.
CREATE INDEX IF NOT EXISTS idx_player_first_match_summary_user
    ON player_first_match_summary (user_id);

CREATE INDEX IF NOT EXISTS idx_player_activity_daily_game_day
    ON player_activity_daily (game_id, day);
