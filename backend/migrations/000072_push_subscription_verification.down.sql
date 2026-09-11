DROP INDEX IF EXISTS idx_push_subscriptions_user_verified;
DROP INDEX IF EXISTS idx_push_subscriptions_verification_token;
ALTER TABLE push_subscriptions
    DROP COLUMN IF EXISTS verification_sent_at,
    DROP COLUMN IF EXISTS verification_token,
    DROP COLUMN IF EXISTS verified_at;
