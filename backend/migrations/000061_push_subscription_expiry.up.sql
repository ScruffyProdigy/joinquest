-- JQ-198: keep expired push subscriptions instead of deleting them.
--
-- The seat-hold window (JQ-199) tiers an absent player by whether push can
-- reach them, and those tier values are meant to be tuned on production data.
-- Tuning needs to distinguish a player who NEVER opted in (a UX problem on the
-- waiting page) from one who opted in and whose subscription later expired (a
-- storage-invalidation problem). Hard-deleting on the push service's 404/410
-- collapsed both into "no rows" and made them indistinguishable forever.
--
-- Retention is deliberately unbounded for now: one narrow row per dead browser
-- install is cheap next to losing the signal. Revisit if it grows.
ALTER TABLE push_subscriptions
    ADD COLUMN IF NOT EXISTS expired_at TIMESTAMPTZ;

-- Reachability reads only live rows, and does so on every hold decision.
-- Partial index so the expired tail never costs the hot path anything.
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user_live
    ON push_subscriptions (user_id)
    WHERE expired_at IS NULL;
