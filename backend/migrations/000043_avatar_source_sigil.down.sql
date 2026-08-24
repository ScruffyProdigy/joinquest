-- Guests holding a sigil lose their avatar, since the old constraint has no room for it.
UPDATE users SET avatar_key = NULL, avatar_source = NULL, avatar_url = NULL
WHERE avatar_source = 'sigil';

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_avatar_source_check;

ALTER TABLE users
    ADD CONSTRAINT users_avatar_source_check
    CHECK (avatar_source IN ('starter', 'spirit_animal'));
