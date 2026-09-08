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

-- Give the eligibility fixture game (000038) a mode that actually asks for a
-- deck, so the picker — including a locked choice with its unlock progress —
-- can be seen locally against `go run ./cmd/fixturegame`, the same way the
-- locked-mode UI already can.
UPDATE game_modes
SET pre_queue = '{"groups":[{"key":"deck","kind":"Deck","label":"Bring a deck","min":1,"max":1}]}'::jsonb
WHERE id = 'b2000000-0000-4000-8000-000000000003';
