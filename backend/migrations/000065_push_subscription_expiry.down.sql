DROP INDEX IF EXISTS idx_push_subscriptions_user_live;
ALTER TABLE push_subscriptions DROP COLUMN IF EXISTS expired_at;
