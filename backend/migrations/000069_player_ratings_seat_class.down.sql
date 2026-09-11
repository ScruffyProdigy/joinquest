-- Drop the per-seat rows before narrowing the key: they are duplicates of the
-- mode-level row under the old primary key, and a replay rebuilds them.
DELETE FROM player_ratings WHERE seat_class <> '';

ALTER TABLE player_ratings
    DROP CONSTRAINT IF EXISTS player_ratings_pkey;

ALTER TABLE player_ratings
    ADD PRIMARY KEY (user_id, game_id, mode_key);

ALTER TABLE player_ratings
    DROP COLUMN IF EXISTS seat_class;
