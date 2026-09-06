-- The column cannot go back to NOT NULL while anyone is nameless, so rebuild a
-- placeholder for them. There is no unique index on display_name, so the
-- digits only need to be stable, not distinct.
UPDATE users
SET display_name = 'guest#' || lpad(((abs(hashtext(id::text)) % 900000) + 100000)::text, 6, '0')
WHERE display_name IS NULL;

ALTER TABLE users ALTER COLUMN display_name SET NOT NULL;
