-- Wait estimates are now bucketed per role (JQ-242): a composition mode splits
-- its queue by queue_path, and those lines move at genuinely different speeds.
-- The read partitions by (mode_queue_id, queue_path) and takes the most recent
-- fills of each, so the 000054 index — which knew nothing about queue_path —
-- leaves the per-path ordering to a sort.
--
-- Replaces rather than edits 000054 so a database that already ran it ends up
-- with the right index either way.
DROP INDEX IF EXISTS idx_game_queues_fill_history;

CREATE INDEX IF NOT EXISTS idx_game_queues_fill_history_by_path
    ON game_queues (mode_queue_id, queue_path, matched_at DESC)
    WHERE matched_at IS NOT NULL;
