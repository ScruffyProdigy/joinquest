-- Indexes for the catalog card's live player counts.
-- The counts are grouped aggregates over only the rows that are currently live, so both
-- indexes are partial: they stay tiny no matter how much finished history the tables accumulate.
CREATE INDEX IF NOT EXISTS idx_game_sessions_active_game_id
    ON game_sessions (game_id, started_at)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_game_queues_waiting_game_id
    ON game_queues (game_id)
    WHERE status = 'waiting';
