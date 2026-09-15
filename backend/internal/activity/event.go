// Package activity records what players did, so that a later ticket can work out
// which of it predicts how hard a game is to get into.
//
// The platform wants to tell players how difficult a game is along two axes: the
// floor (how hard it is to start) and the ceiling (how much room there is to
// master). The floor matters more, and nobody yet knows which measurable signals
// track it. JQ-143's answer is to record broadly now and correlate later -- this
// package is the recording, and the correlation is deliberately somebody else's
// ticket.
//
// Two properties make "record everything" safe here rather than a liability.
//
// First, the envelope. An Event is a type, an actor, a game, a mode, a session, a
// timestamp and a free JSON payload -- not a wide struct of guessed fields. Adding
// a signal later costs a constant in this file and nothing in the database.
//
// Second, and more important: recording cannot fail a player-facing action. Record
// takes no context and returns no error, so there is nothing for a caller to block
// on, wait for, or mishandle. Matchmaking cannot be broken by instrumentation that
// a caller is structurally incapable of noticing. See Writer for what that costs.
package activity

import (
	"time"

	"github.com/google/uuid"
)

// Known event types. These are plain strings rather than a database enum or a
// CHECK constraint precisely so that adding one is a one-line change here: the
// table stores whatever it is given, and a new signal never needs a migration.
//
// Every type below is derivable from something the lobby already knows, which is
// why they are the ones worth having on day one.
const (
	// A player joined a mode queue. The start of the funnel, and the denominator
	// for everything downstream.
	EventQueueJoined = "queue_joined"

	// A player left a queue before any match formed -- deliberately, or by being
	// evicted after their socket stayed shut past the grace window. The payload's
	// "reason" tells those apart, and they mean quite different things: one is a
	// decision, the other may just be a bad connection.
	//
	// Deliberately keyed off the server's eviction decision rather than the socket
	// edge itself. A client that drops and reconnects inside the grace window never
	// abandoned anything, and counting those would read as difficulty when it is
	// really just a flaky network -- a distortion that grows with the client's
	// reconnect budget (JQ-283 raised that budget to ~87.5s).
	EventQueueAbandoned = "queue_abandoned"

	// A match this player is in was created. Emitted from session creation rather
	// than from a table being stamped started: a matchmade session can involve zero
	// tables (all catalog joiners) or several (a 3v3 formed out of two rooms), so
	// hooking the table would both miss and double-count.
	EventMatchStarted = "match_started"

	// The game reported back that it is ready for players.
	EventMatchProvisioned = "match_provisioned"

	// A player asked for their launch URL. This is the closest the platform can get
	// to "the player actually launched", and it is an upper bound rather than a
	// confirmation -- see the honesty note on EventMatchFinished.
	EventLaunchURLRequested = "launch_url_requested"

	// The game reported how one player's match ended, carrying a reason. FORFEIT and
	// DISCONNECT on a player's first-ever match of a game are close to a direct
	// reading of its floor, which is the single sharpest hypothesis the ticket has.
	//
	// The honesty rule for this type and its neighbours: these events say what the
	// platform observed, never what it inferred. If a game never reports, the match
	// has an unknown outcome -- not a failed one. Nothing in this package invents an
	// outcome to fill that gap.
	EventMatchFinished = "match_finished"

	// The session as a whole ended.
	EventMatchCompleted = "match_completed"
)

// SourceLobby marks an event the lobby observed itself.
//
// It is a field rather than an assumption because games knowing things the lobby
// never will -- that a player fumbled a tutorial, say -- is a real possibility the
// ticket raises and defers to a later protocol change. Carrying the column now
// costs nothing and keeps the envelope from precluding it.
const SourceLobby = "lobby"

// Event is one thing that happened, in the extensible envelope JQ-143 describes.
//
// The pointer fields are genuinely optional, not merely convenient: a queue that
// never formed a match has no session, and a sweep-initiated event may have no
// single actor. A zero uuid would claim an entity that does not exist, so absence
// is spelled as absence.
type Event struct {
	Type   string
	Source string

	// The lobby user id, and per the ticket's retention rule the only identifying
	// value permitted anywhere on this event. Payload must not carry another.
	UserID    *uuid.UUID
	GameID    *uuid.UUID
	ModeKey   string
	SessionID *uuid.UUID

	// When the thing described actually happened, which is frequently not when this
	// event was constructed. Where a database clock already holds the honest answer
	// -- game_sessions.started_at, for instance -- callers pass that through rather
	// than stamping their own process's clock. Zero means "now", resolved by Writer.
	OccurredAt time.Time

	// Whatever this event type knows. Ids, enums, counts and durations only; see
	// sanitizePayload, which enforces that rather than trusting it.
	Payload map[string]any
}

// Recorder accepts events. Implementations must not block and must not fail.
//
// The signature is the contract: no context to cancel, no error to check. A caller
// on the matchmaking path physically cannot wait on this or be broken by it, which
// is what the acceptance criterion "writing an event never blocks or fails a
// player-facing action" actually demands. Any implementation that wants to do real
// work has to do it somewhere else -- which is exactly what Writer does.
type Recorder interface {
	Record(Event)
}

// Nop discards every event. It is the default a Store gets when nobody wires a
// real recorder, so tests and one-off tools run without an events table.
type Nop struct{}

// Record discards the event.
func (Nop) Record(Event) {}
