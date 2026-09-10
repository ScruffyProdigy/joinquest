ALTER TABLE forming_matches
    DROP COLUMN IF EXISTS held_user_id,
    DROP COLUMN IF EXISTS hold_started_at;
