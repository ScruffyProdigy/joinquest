-- Records when a player actually picked their display name, so nothing has to
-- guess it from the string. Two placeholder shapes were being pattern-matched
-- before this: "guest#NNNNNN" from RandomGuestDisplayName, and the " (new)"
-- suffix migration 000002 appended to names derived from an email address.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS display_name_chosen_at TIMESTAMPTZ;

-- Everyone whose name is not a placeholder chose it at some point; created_at
-- is the best evidence we have of when.
UPDATE users
SET display_name_chosen_at = created_at
WHERE display_name_chosen_at IS NULL
  AND display_name !~ '^guest#[0-9]+$'
  AND display_name NOT LIKE '% (new)'
  AND btrim(display_name) <> '';

-- The " (new)" marker lived in the stored name itself. Now that the column
-- carries the fact, drop the suffix so the name is just a name.
UPDATE users
SET display_name = btrim(regexp_replace(display_name, ' \(new\)$', ''))
WHERE display_name LIKE '% (new)';
