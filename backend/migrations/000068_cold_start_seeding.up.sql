-- JQ-154: cold-start seeding from another mode of the same game.
--
-- Two tables, and neither of them is a rating. player_ratings stays exactly
-- what it was — a cache of a replay over rating_match_inputs — because a seed
-- written into it would be erased by the next replay and would also blur the
-- line between what a player has shown and what we guessed about them. A seed
-- is derived on read from the row below and the player's ratings elsewhere in
-- the game, so it needs no invalidation and disappears of its own accord the
-- moment a real rating exists.

-- One row per ordered pair of modes within a game: how well the source mode
-- predicts the target one, and how badly it missed when that prediction was
-- held out.
--
-- Ordered, not unordered. The regression coefficient and the residual differ
-- by direction — a tightly-clustered mode predicts a widely-spread one far
-- less precisely than the reverse — so arena->duel and duel->arena are two
-- measurements and get two rows.
--
-- Text mode keys rather than a game_modes FK, matching player_ratings and for
-- the same reason: mode rows are deleted and rewritten on manifest sync, and
-- the natural key (game_id, mode_key) is what survives that.
CREATE TABLE IF NOT EXISTS mode_pair_correlations (
    game_id          UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    source_mode_key  TEXT NOT NULL,
    target_mode_key  TEXT NOT NULL,

    -- How many players held converged ratings in both modes at fit time. The
    -- gate (internal/coldstart.MinPairedPlayers) is applied again at serve
    -- time from this column, so a row that was usable when written and is not
    -- any more cannot keep serving seeds.
    paired_players   INT NOT NULL,
    correlation      DOUBLE PRECISION NOT NULL,

    -- The paired population's moments in each mode. All four are needed to
    -- map a player's standing in one distribution onto the other, which is
    -- what lets two modes on different scales still compose.
    source_mean      DOUBLE PRECISION NOT NULL,
    source_sd        DOUBLE PRECISION NOT NULL,
    target_mean      DOUBLE PRECISION NOT NULL,
    target_sd        DOUBLE PRECISION NOT NULL,

    -- Held-out error, cross-validated over players kept out of the fit.
    -- residual_sd is where a seed's sigma comes from; flat_residual_sd is the
    -- same error for the do-nothing baseline, stored so "does seeding beat
    -- the flat prior here?" is answerable from the row rather than by
    -- refitting. A row whose residual_sd is not below its flat_residual_sd is
    -- retained and refused at serve time: "measured, and it does not work
    -- here" is a useful answer, and a missing row cannot be told apart from a
    -- recompute that never ran.
    residual_sd      DOUBLE PRECISION NOT NULL,
    flat_residual_sd DOUBLE PRECISION NOT NULL,

    -- Reported, never corrected for. A systematic overshoot is a fact about
    -- the fit that a human should see; subtracting it would hide a broken
    -- pair behind a plausible centre.
    bias             DOUBLE PRECISION NOT NULL,

    -- The error at the median and 90th-percentile player. residual_sd alone
    -- reads as reassuring for a pair that is mostly fine and badly wrong for
    -- a tail, and that tail is who a mis-seed hurts.
    abs_error_p50    DOUBLE PRECISION NOT NULL,
    abs_error_p90    DOUBLE PRECISION NOT NULL,

    computed_at      TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (game_id, source_mode_key, target_mode_key),
    CONSTRAINT mode_pair_correlations_distinct_modes CHECK (source_mode_key <> target_mode_key)
);

-- Serve-time lookup: every source mode that can seed one target mode.
CREATE INDEX IF NOT EXISTS idx_mode_pair_correlations_target
    ON mode_pair_correlations (game_id, target_mode_key);

-- One row per player per mode they were seeded into: the audit trail for a
-- claim the lobby made about someone before it had watched them play.
--
-- When a mode's matchmaking turns out lopsided, the first question is which
-- players were seeded, from where, and on how strong a measurement — and the
-- scheduled recompute will by then have moved the numbers in
-- mode_pair_correlations, so the measurement is copied here rather than
-- referenced. correlation and paired_players are what they were at the moment
-- of seeding.
--
-- Keyed on the seeded (user, game, mode) so it records the first seed served
-- for that mode and stays one row however many times the seed is read. That
-- first seed is the one worth auditing: it is what the player's earliest
-- matches were built on, and every later read before their first result
-- returns the same numbers anyway.
CREATE TABLE IF NOT EXISTS rating_seed_events (
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id          UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    mode_key         TEXT NOT NULL,

    source_mode_key  TEXT NOT NULL,
    source_mu        DOUBLE PRECISION NOT NULL,

    seeded_mu        DOUBLE PRECISION NOT NULL,
    seeded_sigma     DOUBLE PRECISION NOT NULL,

    correlation      DOUBLE PRECISION NOT NULL,
    paired_players   INT NOT NULL,

    seeded_at        TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (user_id, game_id, mode_key)
);

-- "Which players did we seed into this mode, and when did that start?" — the
-- question an investigation into a lopsided mode opens with.
CREATE INDEX IF NOT EXISTS idx_rating_seed_events_mode
    ON rating_seed_events (game_id, mode_key, seeded_at);
