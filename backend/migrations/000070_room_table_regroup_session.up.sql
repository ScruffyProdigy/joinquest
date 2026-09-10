-- Table.regroupRoster asks "which finished match is this table regrouping from?", but the
-- only pointer was game_sessions.regroup_table_id, which answers the opposite direction.
-- Reading it backwards cost a game_sessions query for every table on every room render and
-- every tableUpdated push, including the overwhelming majority of tables that never came
-- from a match at all (JQ-177).
--
-- The forward pointer lives here instead. It is written by the same two places that stamp
-- regroup_table_id — ClaimRegroupTable for a catalog-origin match, and the post-session
-- reset for a group that regroups at the table it already had — so the two stay in step.
--
-- Overwriting on each match is deliberate and matches what the reverse lookup ordered for:
-- a room's persistent table accumulates one game_sessions row per match, and the one being
-- regrouped from is the latest.
ALTER TABLE room_tables
    ADD COLUMN IF NOT EXISTS regroup_session_id UUID REFERENCES game_sessions(id) ON DELETE SET NULL;

-- Backfill the same answer the reverse lookup would have given: per table, the session with
-- the most recent started_at, id breaking a tie.
UPDATE room_tables t
SET regroup_session_id = s.id
FROM (
    SELECT DISTINCT ON (regroup_table_id) id, regroup_table_id
    FROM game_sessions
    WHERE regroup_table_id IS NOT NULL
    ORDER BY regroup_table_id, started_at DESC NULLS LAST, id DESC
) s
WHERE t.id = s.regroup_table_id
  AND t.regroup_session_id IS DISTINCT FROM s.id;

-- Added by 000049 purely to make the reverse lookup affordable. Nothing reads
-- game_sessions by regroup_table_id any more, so it is now write cost with no reader.
DROP INDEX IF EXISTS idx_game_sessions_regroup_table;
