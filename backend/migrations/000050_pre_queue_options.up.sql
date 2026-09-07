-- Pre-queue options as a platform capability (JQ-163).
--
-- A mode declares its option groups in its manifest; the roster inside them is
-- per player and comes from the game at request time, so nothing here stores
-- choices — only what the mode asks for, and what each player picked.

-- What the mode asks for: {"groups":[{key,kind,label,min,max}]}. NULL means the
-- mode has no pre-queue step, which is almost every mode.
ALTER TABLE game_modes
    ADD COLUMN IF NOT EXISTS pre_queue JSONB;

-- What a player picked: [{"groupKey":"helpers","optionIds":["ferrus","tempered"]}].
-- The same shape rides all three tables because the same selection travels the
-- whole way: claimed at the queue or at a table seat, carried onto the session
-- participant, and handed to the game in the provision payload.
ALTER TABLE game_queues
    ADD COLUMN IF NOT EXISTS queue_options JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE table_seats
    ADD COLUMN IF NOT EXISTS queue_options JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE game_session_participants
    ADD COLUMN IF NOT EXISTS queue_options JSONB NOT NULL DEFAULT '[]'::jsonb;
