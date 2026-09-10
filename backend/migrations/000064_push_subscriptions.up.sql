-- Web Push subscriptions, so a queued player who left the page can be told
-- their match is ready.
--
-- One row per browser install, not per user: a player may queue from a phone
-- and a desktop at once and both should ring.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- The push service URL, and the identity of the install. Unique across
    -- users: a re-signed-in browser moves to the new owner.
    endpoint    TEXT NOT NULL UNIQUE,
    -- ECDH public key and auth secret, base64url -- the form encryption needs.
    p256dh      TEXT NOT NULL,
    auth        TEXT NOT NULL,
    -- Diagnostic only: which platforms actually subscribe.
    user_agent  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Set on every successful send: "still works" vs "registered once".
    last_used_at TIMESTAMPTZ
);

-- Read on every match formation for every notified user.
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user
    ON push_subscriptions (user_id);
