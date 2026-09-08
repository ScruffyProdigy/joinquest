ALTER TABLE game_session_participants DROP COLUMN IF EXISTS queue_options;
ALTER TABLE table_seats DROP COLUMN IF EXISTS queue_options;
ALTER TABLE game_queues DROP COLUMN IF EXISTS queue_options;
ALTER TABLE game_modes DROP COLUMN IF EXISTS pre_queue;
