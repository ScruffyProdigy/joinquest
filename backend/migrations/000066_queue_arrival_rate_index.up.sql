-- Live wait estimates (JQ-244) measure a line's dynamics over a short recent
-- window: who arrived, who was consumed by a match, and how many wait now.
--
-- The consumption leg is already served by 000058's
-- idx_game_queues_fill_history_by_path, and depth by 000015's
-- idx_game_queues_mode_queue_path_waiting. Arrivals had no index at all —
-- game_queues has never been read by join time before.
--
-- Arrival rate is not a nicety: JQ-142's design settled that deciding whether
-- to hold a dequeue for skill matching turns on it, and on nothing else that
-- queue depth can supply.
CREATE INDEX IF NOT EXISTS idx_game_queues_arrivals_by_path
    ON game_queues (mode_queue_id, queue_path, joined_at DESC);
