-- The fixture eligibility game (migration 000038) was seeded with visibility='public',
-- which makes it eligible for ListCatalogGames (status='active' AND visibility='public'
-- AND an active mode+queue exists) -- so it shows up in the real public games list in
-- every environment, including production, backed by a localhost api_base_url that will
-- never resolve outside local dev. Demote it to 'private_testing' (a valid value per the
-- games_visibility_check constraint added in 000034) so it stops appearing in the public
-- catalog while remaining in the database for local fixture use.

UPDATE games
SET visibility = 'private_testing'
WHERE id = 'b1000000-0000-4000-8000-000000000001';
