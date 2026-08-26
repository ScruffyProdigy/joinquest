-- Guest-tier sigils are a third avatar source, alongside starter icons and spirit animals.
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_avatar_source_check;

ALTER TABLE users
    ADD CONSTRAINT users_avatar_source_check
    CHECK (avatar_source IN ('starter', 'spirit_animal', 'sigil'));
