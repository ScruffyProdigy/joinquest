-- Keep expired push subscriptions instead of deleting them, so a player who
-- never opted in stays distinguishable from one whose subscription died. The
-- two have different fixes, and the seat-hold tiers are tuned on that.
--
-- Retention is unbounded for now; revisit if it grows.
ALTER TABLE push_subscriptions
    ADD COLUMN IF NOT EXISTS expired_at TIMESTAMPTZ;

-- Partial: reachability reads only live rows, on every hold decision.
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user_live
    ON push_subscriptions (user_id)
    WHERE expired_at IS NULL;
