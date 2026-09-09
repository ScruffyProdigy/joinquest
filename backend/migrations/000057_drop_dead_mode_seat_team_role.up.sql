-- `team` and `role` were added with the catalog in 000007, before seat templates
-- existed. Nothing has ever written them: the only writer, the manifest sync in
-- store/catalog.go, inserted literal NULLs, and production confirms every row is
-- NULL. Seat grouping now lives on the columns 000014 added — `affinity_key`
-- holds the outer grouping (e.g. `Team:1`), `queue_path` the leaf role bucket —
-- so these two would only duplicate it.

ALTER TABLE game_mode_seats
    DROP COLUMN IF EXISTS team,
    DROP COLUMN IF EXISTS role;
