-- A display name the player never chose used to be faked: "guest#NNNNNN" for
-- guests, the email local part plus a " (new)" suffix for signups. Both had to
-- be pattern-matched to be recognised. Absence says it directly instead.
ALTER TABLE users ALTER COLUMN display_name DROP NOT NULL;

UPDATE users
SET display_name = NULL
WHERE display_name ~ '^guest#[0-9]+$'
   OR display_name LIKE '% (new)'
   OR btrim(display_name) = '';
