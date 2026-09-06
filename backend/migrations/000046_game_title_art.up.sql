-- Catalog card wordmarks: the transparent title mark composited over the card art,
-- plus the 4:3 catalog heroes the new card slot is cut for.
--
-- The mark is stored rather than derived from the slug so it can be swapped per
-- game (and, later, per viewer language) without a code change. Placement varies
-- per mark -- a wide lockup wants the full card width, a compact one a third of
-- it -- so anchor and width ride along with the URL. Insets and the height cap are
-- constant across the design and live in the frontend.
ALTER TABLE games
    ADD COLUMN IF NOT EXISTS title_url TEXT,
    ADD COLUMN IF NOT EXISTS title_anchor TEXT,
    ADD COLUMN IF NOT EXISTS title_width_pct REAL;

ALTER TABLE games
    DROP CONSTRAINT IF EXISTS games_title_anchor_check;

ALTER TABLE games
    ADD CONSTRAINT games_title_anchor_check CHECK (
        title_anchor IS NULL
        OR title_anchor IN ('top-left', 'top-center', 'bottom-left', 'bottom-center', 'middle-center')
    );

-- Catalog cards are 4:3 and full-bleed. The .jpg heroes are cut 5:2 for the old
-- banner slot, so point the catalog-only hero at the 4:3 art already in the repo
-- and leave hero_url alone for the detail page.
UPDATE games
SET
    catalog_hero_url = '/games/word-hunt-hero.webp',
    title_url = '/games/word-hunt-title.webp',
    title_anchor = 'bottom-left',
    title_width_pct = 70
WHERE slug = 'word-hunt' OR name ILIKE 'word hunt%';

UPDATE games
SET
    catalog_hero_url = '/games/rpslr-hero.webp',
    title_url = '/games/rpslr-title.webp',
    title_anchor = 'top-left',
    title_width_pct = 35
-- Matched by slug only. Earlier migrations also matched the seed id
-- a1000000-...-0002, which is Party Lobby, not this game -- see 000032, which
-- exists to undo exactly that mistake.
WHERE slug IN ('rock-paper-scissors-lizard-robot', 'rock-paper-scissors-lizard-spock');
