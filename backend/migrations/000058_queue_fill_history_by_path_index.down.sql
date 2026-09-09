DROP INDEX IF EXISTS idx_game_queues_fill_history_by_path;

CREATE INDEX IF NOT EXISTS idx_game_queues_fill_history
    ON game_queues (mode_queue_id, matched_at DESC)
    WHERE matched_at IS NOT NULL;
