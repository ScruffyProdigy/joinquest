-- Migration 000038 inserted the eligibility fixture game's modes without seat_template,
-- which store.GetGameModeByID (unlike ListGameModesByGameID) scans as a non-nullable
-- json.RawMessage column, so any lookup of these modes by id (e.g. GameMode.eligibility)
-- fails with "unsupported Scan ... storing driver.Value type <nil>". Backfill it here,
-- matching the {"count": N} shape used for the other seeded modes (migration 000014).

UPDATE game_modes
SET seat_template = '{"count": 2}'::jsonb
WHERE game_id = 'b1000000-0000-4000-8000-000000000001'
  AND mode_key IN ('arena', 'legendary', 'standard', 'commander');

UPDATE game_modes
SET seat_template = '{"count": 1}'::jsonb
WHERE game_id = 'b1000000-0000-4000-8000-000000000001'
  AND mode_key = 'deck-builder';
