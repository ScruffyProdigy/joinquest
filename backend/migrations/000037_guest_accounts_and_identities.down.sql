DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS user_emails;

ALTER TABLE users DROP COLUMN IF EXISTS merged_into_user_id;
ALTER TABLE users DROP COLUMN IF EXISTS is_guest;

DROP INDEX IF EXISTS idx_users_email_unique;

-- Guest and merged accounts may leave users.email NULL; backfill before NOT NULL.
UPDATE users
SET email = 'rollback+' || id::text || '@inactive.local'
WHERE email IS NULL OR email = '';

ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
