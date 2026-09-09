-- Lossless: the columns never held a value, so there is nothing to restore.
ALTER TABLE game_mode_seats
    ADD COLUMN IF NOT EXISTS team VARCHAR(100),
    ADD COLUMN IF NOT EXISTS role VARCHAR(100);
