ALTER TABLE game_session_participants
    DROP COLUMN IF EXISTS finish_reason,
    DROP COLUMN IF EXISTS placement,
    DROP COLUMN IF EXISTS finish_metadata,
    DROP COLUMN IF EXISTS is_winner,
    DROP COLUMN IF EXISTS regroup_declined_at;

ALTER TABLE game_sessions
    DROP COLUMN IF EXISTS result_status,
    DROP COLUMN IF EXISTS result_metadata,
    DROP COLUMN IF EXISTS result_reported_at,
    DROP COLUMN IF EXISTS regroup_table_id;
