-- JQ-227: performance variance (beta) learned per mode, rather than asserted
-- globally.
--
-- Beta is how much a gap in skill actually predicts who wins. JQ-139 shipped it
-- as one constant for the whole catalog (the engine's sigma / 2), which asserts
-- that skill separates players exactly as well in an RPSLR duel as in a
-- high-luck party game. A row here says otherwise for one mode.
--
-- Absence of a row is the "not measured" state, and it is the common one. It
-- means the mode falls back to the engine's default beta — deliberately not
-- the same thing as a stored row that happens to hold the default value, which
-- would claim a measurement nobody made. A mode with too little history to fit
-- must read as unmeasured, so it gets no row at all.
--
-- Keyed by (game_id, mode_key) text rather than by game_modes.id, matching
-- rating_match_inputs: mode rows come and go with manifest sync and the
-- natural key is what survives. A mode's constants must outlive a re-register,
-- because the history they were fitted against does.
CREATE TABLE IF NOT EXISTS mode_rating_constants (
    game_id      UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    mode_key     TEXT NOT NULL,
    beta         DOUBLE PRECISION NOT NULL CHECK (beta > 0),
    -- The engine identity these constants imply, e.g.
    -- "weng-lin/plackett-luce@1+beta=6.25". Stored rather than derived in SQL
    -- so ListModesNeedingReplay can compare it against player_ratings.engine_id
    -- in one query: a mode whose ratings were computed under different
    -- constants is stale in exactly the way a mode with newer inputs is stale,
    -- and both must trigger the same full replay. Ratings computed under
    -- different betas are not comparable, and mixing them silently would
    -- corrupt the mode's history.
    engine_id    TEXT NOT NULL,
    -- How many held-out matches the fit was selected on. It travels with the
    -- estimate permanently, not just in the report that produced it: a beta
    -- fitted on 200 matches and one fitted on 40,000 are the same number and
    -- not the same evidence.
    sample_size  INT NOT NULL CHECK (sample_size > 0),
    estimated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (game_id, mode_key)
);
