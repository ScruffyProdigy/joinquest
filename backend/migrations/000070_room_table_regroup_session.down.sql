CREATE INDEX IF NOT EXISTS idx_game_sessions_regroup_table
    ON game_sessions (regroup_table_id, started_at DESC)
    WHERE regroup_table_id IS NOT NULL;

ALTER TABLE room_tables DROP COLUMN IF EXISTS regroup_session_id;
