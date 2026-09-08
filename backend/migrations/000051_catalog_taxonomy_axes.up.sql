-- JQ-162: split the flat catalog tag list into axes that each answer one question.
--
-- The old `games.tags` mixed four axes (format, duration, genre, difficulty) in
-- one list, so a game's tags could not be relied on to say anything in
-- particular. Genre and difficulty are properties of the *game*; social shape is
-- a property of a *mode* — Word Hunt's Arena and Duel modes differ on exactly
-- that, and a game-level tag cannot express it.
--
-- `games.tags` is kept (deprecated, no longer writable) so this deploy is
-- reversible and so JQ-161 can seed mode durations from which games were `quick`.

ALTER TABLE games
    ADD COLUMN IF NOT EXISTS genre TEXT,
    ADD COLUMN IF NOT EXISTS difficulty TEXT;

ALTER TABLE game_modes
    ADD COLUMN IF NOT EXISTS social_mode TEXT;

ALTER TABLE games ADD CONSTRAINT games_genre_check
    CHECK (genre IS NULL OR genre IN (
        'action', 'strategy', 'deduction', 'words-trivia', 'drawing-creative', 'puzzle'));

ALTER TABLE games ADD CONSTRAINT games_difficulty_check
    CHECK (difficulty IS NULL OR difficulty IN ('casual', 'involved', 'demanding'));

ALTER TABLE game_modes ADD CONSTRAINT game_modes_social_mode_check
    CHECK (social_mode IS NULL OR social_mode IN (
        'free-for-all', '1v1', 'teams', 'hidden-roles', 'co-op'));

-- Backfill genre. Only `words` and `strategy` carried genre meaning; everything
-- else described some other axis, so those games get NULL rather than a guess —
-- a missing genre pill is honest, an invented one is not.
UPDATE games SET genre = 'words-trivia' WHERE genre IS NULL AND 'words' = ANY(tags);
UPDATE games SET genre = 'strategy' WHERE genre IS NULL AND 'strategy' = ANY(tags);

-- Backfill difficulty. `casual` was already a developer-declared difficulty
-- floor buried in the tag list (JQ-145); promote it rather than re-collect it.
UPDATE games SET difficulty = 'casual' WHERE difficulty IS NULL AND 'casual' = ANY(tags);

-- Backfill social_mode per mode, most specific signal first: a mode whose seats
-- name teams is a team mode regardless of what the game was tagged; a strictly
-- two-player mode is a duel; otherwise fall back to the game's format tag.
-- `hidden-roles` is never inferred — no existing signal distinguishes it.
UPDATE game_modes gm SET social_mode = 'teams'
WHERE gm.social_mode IS NULL
  AND EXISTS (
      SELECT 1 FROM game_mode_seats s
      WHERE s.mode_id = gm.id AND s.team IS NOT NULL AND s.team <> '');

UPDATE game_modes gm SET social_mode = '1v1'
WHERE gm.social_mode IS NULL AND gm.min_players = 2 AND gm.max_players = 2;

UPDATE game_modes gm SET social_mode = 'co-op'
WHERE gm.social_mode IS NULL
  AND EXISTS (SELECT 1 FROM games g WHERE g.id = gm.game_id AND 'cooperative' = ANY(g.tags));

UPDATE game_modes gm SET social_mode = 'free-for-all'
WHERE gm.social_mode IS NULL
  AND EXISTS (
      SELECT 1 FROM games g
      WHERE g.id = gm.game_id
        AND (g.tags && ARRAY['competitive', 'party', '1v1']::text[]));
