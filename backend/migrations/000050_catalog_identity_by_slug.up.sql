-- JQ-203: one source of truth for demo catalog identity.
--
-- Two writers disagreed about which seed row is which game. Migrations
-- 000025-000033 wrote RPSLR's copy onto id ...0001 (RPSLS at the time), and
-- k8s/jobs/patch-game-handoff-urls.yaml later reassigned the *slugs* --
-- ...0001 -> word-hunt, ...0002 -> rock-paper-scissors-lizard-robot -- without
-- moving the copy with them. The row advertised at /games/word-hunt therefore
-- wore RPSLR's name, art and tags while handing players off to Word Hunt's
-- server, and Word Hunt was absent from the catalog. Migrations run before the
-- job on every deploy and the job is idempotent, so the state was stable and
-- wrong. 000032 is the same class of bug one occurrence earlier.
--
-- From here identity is written once, here, keyed by slug. The job keeps only
-- what its name promises: environment-specific handoff URLs and the mode, seat
-- and queue rows.
--
-- Seed ids are deliberately not reassigned -- ...0002 owns RPSLR's live modes,
-- seats and queues in production, and moving identity between ids would risk
-- live handoff. Fixing the copy per slug avoids that entirely.

-- 1. Pin the slugs by seed id, so this migration reaches the same state on a
--    fresh database as on production (where the job already renamed the rows).
--    Order matters: ...0001 must release the RPSLR slug before ...0002 claims
--    it, or idx_games_slug_unique trips.
UPDATE games
SET slug = 'word-hunt'
WHERE id = 'a1000000-0000-4000-8000-000000000001';

UPDATE games
SET slug = 'rock-paper-scissors-lizard-robot'
WHERE id = 'a1000000-0000-4000-8000-000000000002';

-- 2. Make ...0002 a live catalog row. Moved out of the k8s job, which used to
--    set this alongside the name and description it no longer owns.
UPDATE games
SET
    status = 'active',
    category = 'catalog',
    manifest_hash = COALESCE(manifest_hash, 'seed-rps-duel-v1'),
    manifest_synced_at = COALESCE(manifest_synced_at, NOW()),
    game_version = COALESCE(game_version, '1.0.0')
WHERE id = 'a1000000-0000-4000-8000-000000000002';

-- 3. Word Hunt's own identity. Recovered from migration history: 000026 (short
--    description, tags), 000030 (icon), 000033 (hero), 000032 (long copy,
--    how-to-play, tutorial), 000046 (catalog art and wordmark).
UPDATE games
SET
    name = 'Word Hunt',
    short_description = 'Everyone competes on a shared word grid. Push-your-luck party scoring.',
    description = 'Word Hunt is a competitive party word game on a shared grid. Clue-givers give one-word hints; everyone else races to guess words tile by tile. The pot grows while you wait — pass early for the biggest score, or push your luck for one more guess.',
    how_to_play = 'Each round a clue-giver picks a one-word clue tied to words on the board. Other players guess which tiles match; correct guesses score from a shared pot that grows over time. Wrong guesses can crash the pot for that clue. First color to clear all their words wins the match.',
    tutorial_url = 'https://word-hunt-arena.win',
    icon_url = '/games/word-hunt-icon.png',
    hero_url = '/games/word-hunt-hero.jpg',
    catalog_hero_url = '/games/word-hunt-hero.webp',
    title_url = '/games/word-hunt-title.webp',
    title_anchor = 'bottom-left',
    title_width_pct = 70,
    tags = ARRAY['party', 'competitive', 'words']
WHERE slug = 'word-hunt';

-- 4. RPSLR's own identity, from the same history: 000025 (short description,
--    tags), 000029 (name), 000030 (icon), 000033 (hero), 000032 (long copy,
--    how-to-play, tutorial), 000046 (catalog art and wordmark).
UPDATE games
SET
    name = 'Rock Paper Scissors Lizard Robot',
    short_description = 'Best-of-five duel with move cooldown. Fast reads, big throws.',
    description = 'Rock Paper Scissors Lizard Robot is a best-of-five duel with a move cooldown twist. Standard RPSLR rules apply, but each move sits out for a few rounds after you play it — so you cannot spam the same throw.',
    how_to_play = 'Pick rock, paper, scissors, lizard, or robot each round. Each move beats two others (rock crushes scissors and lizard, paper covers rock and disproves robot, etc.). After you play a move it gains delay marks and cannot be chosen again until they tick down. Lizard and robot start on cooldown. First to three round wins takes the match.',
    tutorial_url = 'https://rpsls-duel.win',
    icon_url = '/games/rpslr-icon.png',
    hero_url = '/games/rpslr-hero.jpg',
    catalog_hero_url = '/games/rpslr-hero.webp',
    title_url = '/games/rpslr-title.webp',
    title_anchor = 'top-left',
    title_width_pct = 35,
    tags = ARRAY['competitive', '1v1', 'quick']
WHERE slug = 'rock-paper-scissors-lizard-robot';

-- api_base_url is intentionally not set here. It is environment-specific: the
-- k8s job points each row at its production host, and local development leaves
-- ...0001 on http://localhost:3001 for the one demo game server that runs
-- locally (see backend/internal/store/queue_testutil.go). The production
-- mapping is asserted by scripts/check-catalog-identity.mjs.
