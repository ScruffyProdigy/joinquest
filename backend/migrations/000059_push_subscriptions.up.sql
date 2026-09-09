-- JQ-198: Web Push subscriptions, so a queued player who left the page can be
-- told their match is ready.
--
-- One row per browser install, not per user: a player may queue from a phone
-- Home Screen install and a desktop tab at once, and both should ring. The
-- endpoint URL the push service hands us is globally unique and is the identity
-- of that install, so it is the natural key -- re-subscribing in the same
-- browser returns the same endpoint and must update the row rather than
-- accumulate duplicates.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- The push service URL. Unique across users on purpose: if a browser is
    -- handed to a different account, the endpoint moves with the install and
    -- the old owner must lose it, or we would push one person's match to
    -- another person's phone.
    endpoint    TEXT NOT NULL UNIQUE,
    -- ECDH public key and auth secret from the PushSubscription, base64url.
    -- Stored as text because that is the wire form the encryption needs.
    p256dh      TEXT NOT NULL,
    auth        TEXT NOT NULL,
    -- Diagnostic only. Which platforms actually subscribe is the open question
    -- on the ticket (the iOS/Android split), and this is the cheapest way to
    -- answer it without adding analytics.
    user_agent  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Set on every successful send. A subscription is only proof of
    -- reachability until the push service says otherwise, so this is what
    -- distinguishes "registered once" from "still works".
    last_used_at TIMESTAMPTZ
);

-- The hot path: "can we reach this player, and where". Read on every match
-- formation for every notified user, so it must not be a sequential scan.
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user
    ON push_subscriptions (user_id);
