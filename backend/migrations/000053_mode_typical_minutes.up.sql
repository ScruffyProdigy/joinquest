-- JQ-161: a mode's typical session length, so the catalog card can say "12 min".
--
-- On the mode rather than the game, for the same reason social_mode is: Word
-- Hunt's Arena runs about 12 minutes and its Duel about 5, and a game-level
-- number cannot say that. A single typical value, not a range — the prototype
-- carries one integer per mode (`typicalMinutes`) and this column takes its
-- name so the two stay legible against each other.
--
-- Nullable, and it stays that way. A mode without a declared duration shows no
-- duration pill; that is the honest reading of "the developer has not said".
--
-- Nothing is backfilled from the retired `quick` tag. `quick` asserted "sessions
-- under about ten minutes" — a bound, not a value — so promoting it to a number
-- would paint a precise claim on the card that no developer ever made. This
-- follows 000052, which refused to guess a genre for the same reason.

ALTER TABLE game_modes
    ADD COLUMN IF NOT EXISTS typical_minutes INTEGER;

-- The ceiling is deliberately loose. A day is far past any session a lobby can
-- hold players through, so it rejects what cannot be a duration at all without
-- second-guessing an unusually long game. It is not a unit check — a mode that
-- sends 12 minutes as 720 seconds lands inside the range.
ALTER TABLE game_modes ADD CONSTRAINT game_modes_typical_minutes_check
    CHECK (typical_minutes IS NULL OR (typical_minutes >= 1 AND typical_minutes <= 1440));
