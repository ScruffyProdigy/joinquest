-- Persist the outcome games already report via reportPlayerFinished / reportMatchResult,
-- plus the single regroup table a finished match converges on (JQ-135).

ALTER TABLE game_sessions
    ADD COLUMN IF NOT EXISTS result_status TEXT,
    ADD COLUMN IF NOT EXISTS result_metadata JSONB,
    ADD COLUMN IF NOT EXISTS result_reported_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS regroup_table_id UUID REFERENCES room_tables(id) ON DELETE SET NULL;

ALTER TABLE game_session_participants
    ADD COLUMN IF NOT EXISTS finish_reason TEXT,
    ADD COLUMN IF NOT EXISTS placement INT,
    ADD COLUMN IF NOT EXISTS finish_metadata JSONB,
    ADD COLUMN IF NOT EXISTS is_winner BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS regroup_declined_at TIMESTAMP WITH TIME ZONE;
