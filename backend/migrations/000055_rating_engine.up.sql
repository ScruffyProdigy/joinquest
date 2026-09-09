-- JQ-139: skill ratings, derived from an append-only log of match inputs.
--
-- rating_match_inputs is the source of truth. The two rating tables are caches
-- of a replay over it, so any rating can be recomputed and none is the only
-- copy of itself. Sides are snapshotted at match time rather than resolved at
-- replay time, because replaceModeSeatsFromLeavesTx deletes and rewrites every
-- seat row on manifest sync — resolving later would re-side old matches against
-- a newer template.
CREATE TABLE IF NOT EXISTS rating_match_inputs (
    session_id     UUID PRIMARY KEY REFERENCES game_sessions(id) ON DELETE CASCADE,
    game_id        UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    -- Text, not a game_modes FK: mode rows are deleted on manifest sync and the
    -- natural key (game_id, mode_key) is what survives.
    mode_key       TEXT NOT NULL,
    -- [{"rank":0,"entrants":[{"key":"player:<uuid>"},{"key":"seat:White"}]}, ...]
    sides          JSONB NOT NULL,
    -- Retained but not read by the engine today. Lets the pre-queue question be
    -- settled later by backtesting rather than guessed at now.
    queue_options  JSONB NOT NULL DEFAULT '[]'::jsonb,
    inputs_version INT NOT NULL DEFAULT 1,
    rated_at       TIMESTAMPTZ NOT NULL
);

-- Replay order. rated_at alone ties, so session_id breaks it: without a total
-- order the recompute is not reproducible.
CREATE INDEX IF NOT EXISTS idx_rating_match_inputs_replay
    ON rating_match_inputs (game_id, mode_key, rated_at, session_id);

CREATE TABLE IF NOT EXISTS player_ratings (
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id        UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    mode_key       TEXT NOT NULL,
    mu             DOUBLE PRECISION NOT NULL,
    sigma          DOUBLE PRECISION NOT NULL,
    matches_played INT NOT NULL DEFAULT 0,
    engine_id      TEXT NOT NULL,
    last_rated_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, game_id, mode_key)
);

-- Scenarios, seat classes and rated pre-queue options. entity_key is namespaced:
-- 'scenario', 'seat:Team/SpyMaster', 'prequeue:color/white'.
CREATE TABLE IF NOT EXISTS nonplayer_ratings (
    game_id        UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    mode_key       TEXT NOT NULL,
    entity_key     TEXT NOT NULL,
    mu             DOUBLE PRECISION NOT NULL,
    sigma          DOUBLE PRECISION NOT NULL,
    matches_played INT NOT NULL DEFAULT 0,
    engine_id      TEXT NOT NULL,
    last_rated_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (game_id, mode_key, entity_key)
);
