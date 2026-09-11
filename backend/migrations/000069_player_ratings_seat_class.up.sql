-- JQ-229: rate a player on the seat they played, not just the mode.
--
-- player_ratings gains a seat_class so one player can hold several estimates
-- in one mode: the mode-level one, plus one per seat class they have sat in.
--
-- The empty string is the mode-level row, not a sentinel for "unknown". Every
-- existing row is a mode-level row and stays exactly what it was, which is why
-- this backfills with a DEFAULT rather than a data migration. NULL was the
-- alternative and is worse here: it would drop out of the primary key, and
-- reading skill for a mode would have to spell `seat_class IS NULL` in every
-- query and get it wrong once.
--
-- Ratings themselves are a cache of a replay over rating_match_inputs (see
-- migration 000055), so nothing is lost if this column is ever recomputed from
-- scratch; per-role rows appear for a mode the first time it is replayed after
-- its matches start recording which seat each player held.
ALTER TABLE player_ratings
    ADD COLUMN IF NOT EXISTS seat_class TEXT NOT NULL DEFAULT '';

ALTER TABLE player_ratings
    DROP CONSTRAINT IF EXISTS player_ratings_pkey;

ALTER TABLE player_ratings
    ADD PRIMARY KEY (user_id, game_id, mode_key, seat_class);
