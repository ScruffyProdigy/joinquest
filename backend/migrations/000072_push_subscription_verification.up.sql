-- A saved subscription is not a reachable one. The browser reports an endpoint
-- it believes works; whether a push actually arrives is only knowable by
-- sending one and hearing back.
--
-- Verification is what turns the first into the second: at opt-in the backend
-- sends a silent push carrying a token, the service worker acks it, and the row
-- is stamped here. JQ-216 reads the stamp instead of guessing, at a moment when
-- the client is by definition disconnected and cannot be asked.
ALTER TABLE push_subscriptions
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ,
    -- Single-use nonce, cleared on ack. Whoever presents it received the push,
    -- which is the whole proof.
    ADD COLUMN IF NOT EXISTS verification_token TEXT,
    -- Bounds how long a token stays redeemable.
    ADD COLUMN IF NOT EXISTS verification_sent_at TIMESTAMPTZ;

-- The ack arrives knowing only the token, so this is the lookup path. Partial:
-- a redeemed token is cleared, and only outstanding ones are ever looked up.
CREATE UNIQUE INDEX IF NOT EXISTS idx_push_subscriptions_verification_token
    ON push_subscriptions (verification_token)
    WHERE verification_token IS NOT NULL;

-- Reachability reads only verified live rows, on every hold decision.
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user_verified
    ON push_subscriptions (user_id)
    WHERE expired_at IS NULL AND verified_at IS NOT NULL;
