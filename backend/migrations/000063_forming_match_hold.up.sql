-- A held chair needs a start time somewhere, and it cannot be derived from
-- user_presence: a player who has been away ten minutes and is then assigned to a
-- table that completes should get their full window from the moment the table
-- completed, not be treated as ten minutes overdue.
--
-- On forming_matches rather than per assignment because at most one chair is ever
-- held at a time. Two simultaneous holds are what would make a "come back and keep
-- your spot" notification falsifiable -- push both, one returns, the other does
-- not, and the returner arrives to find no match -- so the second away player
-- vacates instead of queueing behind the first.
ALTER TABLE forming_matches
    ADD COLUMN hold_started_at TIMESTAMP WITH TIME ZONE,
    -- Whose absence this window is waiting out. The window belongs to the absence,
    -- not to the match: if this player returns and a different one wanders off, the
    -- new absence gets its own full window rather than inheriting the remainder.
    ADD COLUMN held_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
