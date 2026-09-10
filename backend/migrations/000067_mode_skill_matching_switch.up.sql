-- Skill-aware lobby formation (JQ-226) needs a per-mode opt-out.
--
-- Note what this is not: a population threshold. JQ-142's design settles that
-- the arrival-rate gate is already the population test — at low lambda a queue
-- fires immediately and unbiased, so there is nothing to switch off on a thin
-- mode and no launch decision to make.
--
-- This answers a different question. A mode whose skill signal turns out to be
-- weak or meaningless — one where rating says little about whether players
-- enjoy each other — should be able to opt out however busy it is. Default on,
-- because the lambda gate already makes "on" harmless everywhere else.
ALTER TABLE game_modes
    ADD COLUMN IF NOT EXISTS skill_matching_enabled BOOLEAN NOT NULL DEFAULT TRUE;
