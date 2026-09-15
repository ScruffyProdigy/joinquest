-- Kingship belongs to the table, not to whichever seat happens to be oldest.
--
-- table_seats rows are deleted and re-inserted whenever a player changes seats, and
-- wiped wholesale between rounds, so seated_at says when somebody last sat down, not
-- whose table it is. Deriving the king from it handed the table to another player
-- every time the king moved seats.
ALTER TABLE room_tables
    ADD COLUMN king_user_id UUID REFERENCES users (id) ON DELETE SET NULL;

-- Who inherits when the king is not seated. A sequence only ever counts upwards;
-- NOW() on a virtual machine can hand out timestamps that go backwards, which is
-- enough to shuffle the seating order and elect the wrong player.
ALTER TABLE table_seats
    ADD COLUMN seat_seq BIGSERIAL;

-- Tables already open keep the king they have been playing with: the earliest seat is
-- the answer the old derivation was giving, frozen here rather than left to drift.
UPDATE room_tables t
SET king_user_id = (
    SELECT ts.user_id
    FROM table_seats ts
    WHERE ts.table_id = t.id
    ORDER BY ts.seated_at ASC, ts.id ASC
    LIMIT 1
)
WHERE t.king_user_id IS NULL;
