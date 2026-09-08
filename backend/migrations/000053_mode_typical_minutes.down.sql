ALTER TABLE game_modes DROP CONSTRAINT IF EXISTS game_modes_typical_minutes_check;

ALTER TABLE game_modes DROP COLUMN IF EXISTS typical_minutes;
