-- JQ-139: index rating_match_inputs.sides for containment lookups.
--
-- Account merge (carrySourceRatingHistoryTx) scans this table by entrant key
-- once per real signup (every guest-to-account conversion runs MergeUserInto).
-- The only existing index, idx_rating_match_inputs_replay, orders by
-- (game_id, mode_key, rated_at, session_id) and cannot serve a predicate on
-- sides at all, so that scan was a full sequential scan of the match log.
--
-- jsonb_path_ops indexes only support the containment operator (@>), not the
-- existence operators (?, ?&, ?|). That is exactly what the merge queries
-- need, and jsonb_path_ops produces a smaller, faster index than the default
-- GIN ops class for that narrower job.
CREATE INDEX IF NOT EXISTS idx_rating_match_inputs_sides_gin
    ON rating_match_inputs USING GIN (sides jsonb_path_ops);
