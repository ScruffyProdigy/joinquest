-- Put the " (new)" marker back on names that were never chosen, so the old
-- string-matching code can recognise them again.
UPDATE users
SET display_name = display_name || ' (new)'
WHERE display_name_chosen_at IS NULL
  AND display_name !~ '^guest#[0-9]+$'
  AND display_name NOT LIKE '% (new)'
  AND btrim(display_name) <> '';

ALTER TABLE users DROP COLUMN IF EXISTS display_name_chosen_at;
