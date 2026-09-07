-- Regroup opt-in is explicit: a player seated by resetRoomTableAfterSessionTx has not
-- yet said they are playing again (JQ-135).

ALTER TABLE game_session_participants
    ADD COLUMN IF NOT EXISTS regroup_opted_in_at TIMESTAMP WITH TIME ZONE;
