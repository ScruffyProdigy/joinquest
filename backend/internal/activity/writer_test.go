package activity

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/testdb"
)

// newUnstartedWriter builds a Writer without its background goroutine, so a test can
// fill the buffer and observe what Record does when nothing is draining it.
func newUnstartedWriter(buffer int) *Writer {
	return &Writer{
		queue:         make(chan Event, buffer),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
		batchSize:     defaultBatchSize,
		flushInterval: defaultFlushInterval,
		now:           time.Now,
	}
}

// The acceptance criterion this package exists to satisfy: instrumentation must not
// be able to stall a player-facing action. A full buffer is the worst realistic case
// -- the database is gone and nothing is draining -- and Record still has to return.
func TestRecordDoesNotBlockWhenBufferIsFull(t *testing.T) {
	w := newUnstartedWriter(2)

	w.Record(Event{Type: EventQueueJoined})
	w.Record(Event{Type: EventQueueJoined})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			w.Record(Event{Type: EventQueueJoined})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Record blocked on a full buffer; matchmaking could stall behind instrumentation")
	}

	if got := w.Dropped(); got != 1000 {
		t.Errorf("Dropped() = %d, want 1000", got)
	}
}

// A nil Recorder is the state a Store is in before anything wires one up, and it
// must be a no-op rather than a panic -- otherwise adding an emit call to a code
// path would break every tool that constructs a Store without a writer.
func TestNilWriterIsSafe(t *testing.T) {
	var w *Writer
	w.Record(Event{Type: EventQueueJoined})
	if got := w.Dropped(); got != 0 {
		t.Errorf("Dropped() = %d, want 0", got)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

func TestRecordFillsDefaults(t *testing.T) {
	w := newUnstartedWriter(4)
	before := time.Now()

	w.Record(Event{Type: EventQueueJoined})

	got := <-w.queue
	if got.Source != SourceLobby {
		t.Errorf("Source = %q, want %q", got.Source, SourceLobby)
	}
	if got.OccurredAt.Before(before) {
		t.Errorf("OccurredAt = %v, want >= %v", got.OccurredAt, before)
	}
}

// A caller-supplied time is never overwritten. Several events carry a database
// clock -- game_sessions.started_at, for instance -- and substituting this process's
// clock would silently drift by however long the surrounding transaction took.
func TestRecordPreservesCallerSuppliedTime(t *testing.T) {
	w := newUnstartedWriter(4)
	stamp := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	w.Record(Event{Type: EventMatchStarted, OccurredAt: stamp})

	if got := (<-w.queue).OccurredAt; !got.Equal(stamp) {
		t.Errorf("OccurredAt = %v, want %v", got, stamp)
	}
}

func TestRecordDropsUntypedEvents(t *testing.T) {
	w := newUnstartedWriter(4)
	w.Record(Event{})
	if len(w.queue) != 0 {
		t.Fatalf("queued %d events, want 0", len(w.queue))
	}
}

func TestRecordSanitisesPayload(t *testing.T) {
	w := newUnstartedWriter(4)

	w.Record(Event{
		Type:    EventMatchFinished,
		Payload: map[string]any{"reason": "FORFEIT", "display_name": "Ryan"},
	})

	got := <-w.queue
	if _, ok := got.Payload["display_name"]; ok {
		t.Error("display_name reached the queue; the payload rule is not enforced at Record")
	}
	if got.Payload["reason"] != "FORFEIT" {
		t.Errorf("reason = %v, want FORFEIT", got.Payload["reason"])
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	w := NewWriter(nil)
	if err := w.Close(); err != nil {
		t.Fatalf("first Close() = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close() = %v", err)
	}
}

// A writer whose database is unreachable must still accept and discard events
// rather than panicking or blocking its caller.
func TestWriterWithoutDatabaseStillAcceptsEvents(t *testing.T) {
	w := NewWriter(nil)
	t.Cleanup(func() { _ = w.Close() })

	for i := 0; i < 100; i++ {
		w.Record(Event{Type: EventQueueJoined})
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if got := w.Written(); got != 0 {
		t.Errorf("Written() = %d, want 0", got)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", testdb.RequireURL(t))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestWriterPersistsEvents(t *testing.T) {
	db := openTestDB(t)
	w := NewWriter(db)
	t.Cleanup(func() { _ = w.Close() })

	userID := uuid.New()
	gameID := uuid.New()
	sessionID := uuid.New()
	stamp := time.Date(2026, 5, 4, 9, 30, 0, 0, time.UTC)

	w.Record(Event{
		Type:       EventMatchFinished,
		UserID:     &userID,
		GameID:     &gameID,
		ModeKey:    "duel",
		SessionID:  &sessionID,
		OccurredAt: stamp,
		Payload:    map[string]any{"reason": "FORFEIT", "placement": 2},
	})

	// Close drains, so the row is on disk by the time it returns.
	if err := w.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM player_activity_events WHERE user_id = $1`, userID)
	})

	var (
		eventType  string
		source     string
		modeKey    string
		occurredAt time.Time
		payload    []byte
	)
	err := db.QueryRow(`
		SELECT event_type, source, mode_key, occurred_at, payload
		FROM player_activity_events
		WHERE user_id = $1
	`, userID).Scan(&eventType, &source, &modeKey, &occurredAt, &payload)
	if err != nil {
		t.Fatalf("reading back the event: %v", err)
	}

	if eventType != EventMatchFinished {
		t.Errorf("event_type = %q, want %q", eventType, EventMatchFinished)
	}
	if source != SourceLobby {
		t.Errorf("source = %q, want %q", source, SourceLobby)
	}
	if modeKey != "duel" {
		t.Errorf("mode_key = %q, want duel", modeKey)
	}
	if !occurredAt.UTC().Equal(stamp) {
		t.Errorf("occurred_at = %v, want %v", occurredAt.UTC(), stamp)
	}
	if string(payload) == "" || string(payload) == "{}" {
		t.Errorf("payload = %q, want the recorded fields", payload)
	}
	if got := w.Written(); got != 1 {
		t.Errorf("Written() = %d, want 1", got)
	}
}

// Optional columns are genuinely optional: a queue that never formed a match has no
// session, and storing a zero uuid would claim an entity that does not exist.
func TestWriterStoresNullsForAbsentIdentifiers(t *testing.T) {
	db := openTestDB(t)
	w := NewWriter(db)
	t.Cleanup(func() { _ = w.Close() })

	gameID := uuid.New()
	w.Record(Event{Type: EventQueueAbandoned, GameID: &gameID})
	if err := w.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM player_activity_events WHERE game_id = $1`, gameID)
	})

	var userID, sessionID, modeKey *string
	err := db.QueryRow(`
		SELECT user_id::text, session_id::text, mode_key
		FROM player_activity_events
		WHERE game_id = $1
	`, gameID).Scan(&userID, &sessionID, &modeKey)
	if err != nil {
		t.Fatalf("reading back the event: %v", err)
	}
	if userID != nil {
		t.Errorf("user_id = %v, want NULL", *userID)
	}
	if sessionID != nil {
		t.Errorf("session_id = %v, want NULL", *sessionID)
	}
	if modeKey != nil {
		t.Errorf("mode_key = %v, want NULL", *modeKey)
	}
}

// Batching must not lose the tail: more events than one batch, flushed by Close.
func TestWriterFlushesMoreThanOneBatch(t *testing.T) {
	db := openTestDB(t)
	w := NewWriter(db)
	t.Cleanup(func() { _ = w.Close() })

	gameID := uuid.New()
	const count = defaultBatchSize*2 + 5
	for i := 0; i < count; i++ {
		w.Record(Event{Type: EventQueueJoined, GameID: &gameID})
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM player_activity_events WHERE game_id = $1`, gameID)
	})

	var stored int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM player_activity_events WHERE game_id = $1
	`, gameID).Scan(&stored); err != nil {
		t.Fatalf("counting events: %v", err)
	}
	if stored != count {
		t.Errorf("stored %d events, want %d", stored, count)
	}
}
