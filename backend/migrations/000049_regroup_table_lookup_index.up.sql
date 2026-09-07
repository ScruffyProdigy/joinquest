-- Table.regroupRoster resolves the originating match by regroup_table_id, so game_sessions
-- is now read by that column on every table render. Partial on IS NOT NULL: only sessions
-- that actually converged on a table are ever looked up, so the index stays small no matter
-- how much finished history accumulates. started_at matches the query's recency ordering
-- (JQ-135).
CREATE INDEX IF NOT EXISTS idx_game_sessions_regroup_table
    ON game_sessions (regroup_table_id, started_at DESC)
    WHERE regroup_table_id IS NOT NULL;
