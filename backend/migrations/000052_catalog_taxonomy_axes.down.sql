ALTER TABLE game_modes DROP CONSTRAINT IF EXISTS game_modes_social_mode_check;
ALTER TABLE games DROP CONSTRAINT IF EXISTS games_difficulty_check;
ALTER TABLE games DROP CONSTRAINT IF EXISTS games_genre_check;

ALTER TABLE game_modes DROP COLUMN IF EXISTS social_mode;

ALTER TABLE games
    DROP COLUMN IF EXISTS difficulty,
    DROP COLUMN IF EXISTS genre;
