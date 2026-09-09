-- Wait-time estimates (JQ-58) read one queue's recent fills: rows carrying a
-- matched_at, newest first. idx_game_queues_mode_queue_id alone makes that a
-- scan of every row the queue has ever held, which only gets worse as history
-- accumulates — and nothing prunes game_queues.
--
-- Partial on matched_at IS NOT NULL because the estimate never looks at rows
-- without one, and those are the majority on a busy queue.
CREATE INDEX IF NOT EXISTS idx_game_queues_fill_history
    ON game_queues (mode_queue_id, matched_at DESC)
    WHERE matched_at IS NOT NULL;
