-- JQ-143: the readable face of player_activity_events.
--
-- One of the acceptance criteria is that the stream be queryable for analysis
-- without running ad-hoc SQL against production tables. These views are that: the
-- questions the ticket actually asks, answered in the database, so nobody has to
-- rediscover which event_type means what at the moment they are trying to learn
-- something else.
--
-- They live in their own migration rather than alongside the table on purpose.
-- Analysis questions change much faster than the envelope does; a later ticket that
-- reshapes these views should not have to touch the migration that owns the table.
--
-- WHAT THE PLATFORM CAN ACTUALLY OBSERVE -- read this before trusting a number here.
--
-- The lobby sees a player join a queue, sees a match form, sees provisioning finish,
-- and sees the player ask for their launch URL. After that it is blind until the
-- game reports something back. In particular:
--
--   * A launch URL being issued, or even fetched, is NOT proof the player entered
--     the game. They may never have opened it, or opened it and bounced off a
--     loading screen. `launch_url_requests` is the closest observable proxy and it
--     is an upper bound, never a confirmation.
--   * Entering the game is not observable at all today. There is no column for it
--     below, rather than a column that would quietly be a guess.
--   * A completion or a finish reason exists only when the game reported one. A
--     match with no report is an unknown outcome, not an abandoned one and not a
--     completed one -- `matches_outcome_unknown` counts exactly those, and it is a
--     real answer, not a gap in the data.
--
-- The temptation with a summary like this is to infer the missing half so every
-- column has a number in it. That would make the platform's blind spots invisible
-- at precisely the moment someone is deciding whether a game is hard to get into,
-- which is the one question this data exists to inform. Unknown stays unknown.

-- Per (player, game): their first match, how it went, and whether they came back.
--
-- The ticket's sharpest hypothesis about a game's floor lives here. A FORFEIT or a
-- DISCONNECT on someone's very first attempt is close to a direct reading of "this
-- was too hard to get into", and whether a second match ever happened -- and how long
-- it took -- is the behavioural confirmation.
CREATE OR REPLACE VIEW player_activity_first_match AS
WITH starts AS (
    SELECT
        e.user_id,
        e.game_id,
        e.session_id,
        e.occurred_at,
        ROW_NUMBER() OVER (
            PARTITION BY e.user_id, e.game_id
            ORDER BY e.occurred_at, e.id
        ) AS seq
    FROM player_activity_events e
    WHERE e.event_type = 'match_started'
      AND e.user_id IS NOT NULL
      AND e.game_id IS NOT NULL
)
SELECT
    first_start.user_id,
    first_start.game_id,
    first_start.session_id  AS first_session_id,
    first_start.occurred_at AS first_started_at,

    -- Whether the game ever told us how this player's first match ended. False means
    -- we do not know, which is materially different from "it ended badly" -- see the
    -- observability note above.
    (first_finish.id IS NOT NULL) AS first_outcome_observed,
    first_finish.payload->>'reason' AS first_finish_reason,

    -- Did they come back, and how long did it take. NULL means no second match yet,
    -- which for a recent first match may simply mean not yet.
    second_start.occurred_at AS second_started_at,
    (second_start.occurred_at - first_start.occurred_at) AS gap_to_second_match
FROM starts AS first_start
LEFT JOIN starts AS second_start
       ON second_start.user_id = first_start.user_id
      AND second_start.game_id = first_start.game_id
      AND second_start.seq = 2
LEFT JOIN LATERAL (
    SELECT f.id, f.payload
    FROM player_activity_events f
    WHERE f.event_type = 'match_finished'
      AND f.user_id = first_start.user_id
      AND f.session_id = first_start.session_id
    ORDER BY f.occurred_at, f.id
    LIMIT 1
) AS first_finish ON TRUE
WHERE first_start.seq = 1;

-- Per game: the funnel a developer wants after a playtest, with the platform's blind
-- spots left visible rather than filled in.
CREATE OR REPLACE VIEW game_playtest_summary AS
-- Every column below says its own unit in its name, because the two units here are
-- easy to confuse and produce a plausible wrong answer when they are. A `match_started`
-- event is emitted once per PLAYER, so counting those rows gives player-matches, while
-- a completion is emitted once per SESSION. Dividing one by the other looks like a
-- completion rate and is really the reciprocal of the party size.
WITH per_game AS (
    SELECT
        e.game_id,

        -- Player-scale: one per player action.
        COUNT(*) FILTER (WHERE e.event_type = 'queue_joined')    AS queue_joins,
        COUNT(*) FILTER (WHERE e.event_type = 'queue_abandoned') AS queue_abandons,
        COUNT(*) FILTER (WHERE e.event_type = 'match_started')   AS player_match_starts,
        COUNT(*) FILTER (WHERE e.event_type = 'match_finished')  AS player_finishes_reported,
        COUNT(*) FILTER (
            WHERE e.event_type = 'match_finished' AND e.payload->>'reason' = 'FORFEIT'
        ) AS finishes_forfeit,
        COUNT(*) FILTER (
            WHERE e.event_type = 'match_finished' AND e.payload->>'reason' = 'DISCONNECT'
        ) AS finishes_disconnect,

        -- Match-scale: one per session, however many players were in it.
        COUNT(DISTINCT e.session_id) FILTER (WHERE e.event_type = 'match_started')     AS matches_started,
        COUNT(DISTINCT e.session_id) FILTER (WHERE e.event_type = 'match_provisioned') AS matches_provisioned,
        COUNT(DISTINCT e.session_id) FILTER (WHERE e.event_type = 'match_completed')   AS matches_completed,

        -- Distinct players. Counted rather than summed because a player who asks for
        -- a launch URL twice -- a refresh, a second device -- has still only got as
        -- far as asking once, and the funnel step is "how many players reached here".
        COUNT(DISTINCT e.user_id) FILTER (WHERE e.event_type = 'launch_url_requested') AS players_requesting_launch,
        COUNT(DISTINCT e.user_id) FILTER (WHERE e.event_type = 'match_started')         AS distinct_players,

        MIN(e.occurred_at) AS first_event_at,
        MAX(e.occurred_at) AS last_event_at
    FROM player_activity_events e
    WHERE e.game_id IS NOT NULL
    GROUP BY e.game_id
),
-- Sessions this game started that the game never reported an outcome for. Counted
-- from the session ids themselves rather than subtracted from match_starts: a
-- session has many participants, so start events and completion events are not on
-- the same scale and differencing them would produce a confident wrong number.
unknown_outcomes AS (
    SELECT
        s.game_id,
        COUNT(*) AS matches_outcome_unknown
    FROM (
        SELECT DISTINCT game_id, session_id
        FROM player_activity_events
        WHERE event_type = 'match_started'
          AND game_id IS NOT NULL
          AND session_id IS NOT NULL
    ) AS s
    WHERE NOT EXISTS (
        SELECT 1
        FROM player_activity_events outcome
        WHERE outcome.session_id = s.session_id
          AND outcome.event_type IN ('match_completed', 'match_finished')
    )
    GROUP BY s.game_id
),
-- Players who started a second match of this game at any point. The ticket asks for
-- actual second matches, so this counts observed starts -- not an intent to regroup,
-- and not a rematch offer that was shown.
returning_players AS (
    SELECT
        game_id,
        COUNT(*) AS players_with_second_match
    FROM player_activity_first_match
    WHERE second_started_at IS NOT NULL
    GROUP BY game_id
)
SELECT
    g.game_id,
    g.queue_joins,
    g.queue_abandons,
    g.player_match_starts,
    g.matches_started,
    g.matches_provisioned,

    -- Upper bound on players reaching the game, never a confirmation of entry.
    g.players_requesting_launch,

    g.player_finishes_reported,
    g.finishes_forfeit,
    g.finishes_disconnect,
    g.matches_completed,
    COALESCE(u.matches_outcome_unknown, 0) AS matches_outcome_unknown,
    g.distinct_players,
    COALESCE(r.players_with_second_match, 0) AS players_with_second_match,
    g.first_event_at,
    g.last_event_at
FROM per_game g
LEFT JOIN unknown_outcomes u ON u.game_id = g.game_id
LEFT JOIN returning_players r ON r.game_id = g.game_id;
