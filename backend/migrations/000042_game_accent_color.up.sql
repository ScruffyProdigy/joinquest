-- Optional developer-chosen catalog accent. NULL keeps the slug-hashed palette color.
-- Format is validated in Go (internal/developer.NormalizeAccentColor) so developers
-- get a readable error instead of a constraint violation.
ALTER TABLE games ADD COLUMN IF NOT EXISTS accent_color TEXT;
