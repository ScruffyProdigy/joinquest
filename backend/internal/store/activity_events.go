package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/activity"
)

// Emit helpers for JQ-143's player activity stream.
//
// They live together in one file so that every call site elsewhere in the store is a
// single line. That is not tidiness for its own sake: these calls are threaded
// through matchmaking, the busiest and most contended code in the package, and
// instrumentation that sprawls across those functions makes them harder to read and
// harder to review for the bugs that actually matter.
//
// Two rules hold everywhere below.
//
// Every call happens AFTER the surrounding transaction has committed. An event
// describes something that happened, and a transaction that rolled back did not
// happen; emitting inside the transaction would record matches that never formed.
// Emitting after commit also means an event can never be what aborts a matchmaking
// transaction, which is the acceptance criterion this design is built around.
//
// Nothing here reports what it observed as anything more than what it observed.
// Where the platform is blind -- whether a player who fetched a launch URL actually
// reached the game -- the event says only what happened, and the views leave the
// rest explicitly unknown rather than inferring it.

// recordActivity hands one event to the configured recorder.
func (s *Store) recordActivity(event activity.Event) {
	if s == nil || s.activity == nil {
		return
	}
	s.activity.Record(event)
}

// recordQueueJoined notes a player entering a mode queue: the top of the funnel, and
// the denominator for everything downstream.
func (s *Store) recordQueueJoined(userID uuid.UUID, result *QueueJoinResult) {
	if result == nil {
		return
	}
	// A re-join that changed nothing is not a new join. Counting it would inflate the
	// funnel's first step against a player who only refreshed a page.
	if result.AlreadyInQueue {
		return
	}

	payload := map[string]any{}
	if result.QueuePath != nil {
		payload["queue_path"] = *result.QueuePath
	}
	if result.SwitchedFrom != nil {
		payload["switched_queue"] = true
	}

	gameID := result.GameID
	modeQueueID := result.ModeQueueID
	payload["mode_queue_id"] = modeQueueID.String()

	s.recordActivity(activity.Event{
		Type:    activity.EventQueueJoined,
		UserID:  &userID,
		GameID:  &gameID,
		Payload: payload,
	})
}

// recordQueueAbandoned notes a player leaving a queue before any match formed.
//
// reason distinguishes a deliberate leave from an eviction, and they mean quite
// different things: one is a decision about the game, the other may be nothing but a
// bad connection. An analysis that conflates them would read network trouble as
// difficulty.
func (s *Store) recordQueueAbandoned(userID uuid.UUID, gameID *uuid.UUID, reason string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["reason"] = reason

	s.recordActivity(activity.Event{
		Type:    activity.EventQueueAbandoned,
		UserID:  &userID,
		GameID:  gameID,
		Payload: payload,
	})
}

// recordMatchStarted notes a match forming, once per participant.
//
// Emitted from session creation rather than from a room table being stamped started.
// A matchmade session can involve no tables at all (every player joined from the
// catalog) or several (a 3v3 formed out of two rooms), so hooking the table stamp
// would miss players in the first case and double-count them in the second.
//
// startedAt comes from game_sessions.started_at, a database clock, rather than from
// this process. Postgres stamps it at transaction start, which for "when did this
// match form" is the more honest instant than whenever the commit happened to land
// and this code happened to run.
func (s *Store) recordMatchStarted(gameID uuid.UUID, sessionID uuid.UUID, startedAt time.Time, userIDs []uuid.UUID, origin string) {
	for _, userID := range userIDs {
		userID := userID
		s.recordActivity(activity.Event{
			Type:       activity.EventMatchStarted,
			UserID:     &userID,
			GameID:     &gameID,
			SessionID:  &sessionID,
			OccurredAt: startedAt,
			Payload: map[string]any{
				"origin":            origin,
				"participant_count": len(userIDs),
			},
		})
	}
}

// recordMatchProvisioned notes the game handing back launch URLs for a session.
//
// Paired with the launch-URL request below, it is what "time from match provision to
// the player actually launching" is measured across -- the second half of which the
// platform can only observe as a request, never as an arrival.
func (s *Store) recordMatchProvisioned(sessionID uuid.UUID, seatCount int) {
	s.recordActivity(activity.Event{
		Type:      activity.EventMatchProvisioned,
		SessionID: &sessionID,
		Payload:   map[string]any{"seat_count": seatCount},
	})
}

// recordLaunchURLRequested notes a player asking for their launch URL.
//
// This is the closest the platform can get to "the player launched", and it is an
// upper bound rather than a confirmation: the URL may never be opened, or opened and
// abandoned at a loading screen. Nothing downstream may treat it as proof of entry.
func (s *Store) recordLaunchURLRequested(sessionID, userID uuid.UUID) {
	s.recordActivity(activity.Event{
		Type:      activity.EventLaunchURLRequested,
		UserID:    &userID,
		SessionID: &sessionID,
	})
}

// recordMatchFinished notes the game reporting how one player's match ended.
//
// The finish reason is the sharpest floor signal the ticket identifies: a FORFEIT or
// a DISCONNECT on someone's first-ever match of a game is close to a direct reading
// of how hard that game is to get into.
func (s *Store) recordMatchFinished(sessionID, userID uuid.UUID, reason string, placement *int) {
	payload := map[string]any{"reason": reason}
	if placement != nil {
		payload["placement"] = *placement
	}

	s.recordActivity(activity.Event{
		Type:      activity.EventMatchFinished,
		UserID:    &userID,
		SessionID: &sessionID,
		Payload:   payload,
	})
}

// recordMatchCompleted notes a session ending as a whole.
func (s *Store) recordMatchCompleted(sessionID uuid.UUID, endedAt time.Time) {
	s.recordActivity(activity.Event{
		Type:       activity.EventMatchCompleted,
		SessionID:  &sessionID,
		OccurredAt: endedAt,
	})
}

// gameIDForUserQueueEntry reads the game behind a player's most recent row in one
// mode queue, or nil if there is none.
//
// Needed only on the deliberate-leave path, which is the one abandon site that does
// not already have the game in hand -- an eviction carries it on EvictionResult, and
// a join carries it on QueueJoinResult. One indexed read on an action a player takes
// by hand is a fair price for the per-game funnel being complete; the alternative is
// queue abandons that silently never appear in any game's playtest summary.
//
// Returns nil rather than an error on failure. This is instrumentation: an event
// with no game attached is worth strictly more than a leave that errors because the
// lookup behind its analytics did.
func (s *Store) gameIDForUserQueueEntry(ctx context.Context, modeQueueID, userID uuid.UUID) *uuid.UUID {
	if s == nil || s.db == nil {
		return nil
	}

	var gameID *uuid.UUID
	err := s.db.QueryRowContext(ctx, `
		SELECT game_id
		FROM game_queues
		WHERE mode_queue_id = $1 AND user_id = $2
		ORDER BY joined_at DESC
		LIMIT 1
	`, modeQueueID, userID).Scan(&gameID)
	if err != nil {
		return nil
	}
	return gameID
}
