ALTER TABLE games
    DROP CONSTRAINT IF EXISTS games_title_anchor_check;

ALTER TABLE games
    DROP COLUMN IF EXISTS title_url,
    DROP COLUMN IF EXISTS title_anchor,
    DROP COLUMN IF EXISTS title_width_pct;

UPDATE games
SET catalog_hero_url = NULL
WHERE slug IN ('word-hunt', 'rock-paper-scissors-lizard-robot', 'rock-paper-scissors-lizard-spock');
