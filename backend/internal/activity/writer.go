package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Tuning. Small enough that a stalled database costs bounded memory, large enough
// that a burst of matchmaking does not spill on an idle system.
const (
	// How many events may wait to be written before new ones are dropped. This is
	// the safety valve that makes Record non-blocking: when the database is slow or
	// gone, the queue fills and events are discarded rather than backing pressure up
	// into matchmaking. Losing instrumentation is an acceptable outcome; stalling a
	// player in a queue because an analytics INSERT is slow is not.
	defaultBufferSize = 4096

	// Events per INSERT. This table is append-only on a hot path, so rows are
	// batched into one multi-row statement rather than paying a round trip each.
	defaultBatchSize = 64

	// How long a partial batch waits for company before being written anyway. Low
	// enough that a quiet system still records promptly; anything longer would make
	// the recorded_at lag on a low-traffic game mostly an artifact of this constant.
	defaultFlushInterval = 500 * time.Millisecond
)

// Writer records events into Postgres from a background goroutine.
//
// The split is the point. Record only hands the event to a buffered channel and
// returns -- no I/O, no lock held across a query, no error to propagate. Everything
// that can actually fail (marshalling, the round trip, the database being down)
// happens on the writer's own goroutine, where failing is contained: it logs and
// moves on, and the caller that produced the event has long since continued.
//
// What this costs, stated plainly rather than buried: events are not durable at the
// moment Record returns. A crash loses whatever is still buffered, and a sustained
// stall drops events on the floor by design. That is the right trade for
// instrumentation whose entire purpose is later statistical correlation -- a
// missing fraction of a percent changes no conclusion, whereas a matchmaking path
// that can hang on an analytics write is a live outage.
type Writer struct {
	db    *sql.DB
	queue chan Event

	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}

	dropped atomic.Int64
	written atomic.Int64

	batchSize     int
	flushInterval time.Duration
	now           func() time.Time
}

// NewWriter starts a Writer and its background goroutine. Call Close to drain it.
func NewWriter(db *sql.DB) *Writer {
	w := &Writer{
		db:            db,
		queue:         make(chan Event, defaultBufferSize),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
		batchSize:     defaultBatchSize,
		flushInterval: defaultFlushInterval,
		now:           time.Now,
	}
	go w.run()
	return w
}

// Record queues an event, or drops it if the buffer is full.
//
// Never blocks, never fails, never reports anything to the caller. The default
// branch below is load-bearing: without it a full channel would park the calling
// goroutine, which on the matchmaking path is precisely the failure this design
// exists to rule out.
func (w *Writer) Record(e Event) {
	if w == nil {
		return
	}
	if e.Type == "" {
		return
	}
	if e.Source == "" {
		e.Source = SourceLobby
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = w.now()
	}
	e.Payload = sanitizePayload(e.Payload)

	select {
	case w.queue <- e:
	default:
		// Dropped deliberately. Counted rather than logged: a database outage would
		// otherwise turn one problem into a flood of log lines, and the count is the
		// number an operator actually wants.
		w.dropped.Add(1)
	}
}

// Dropped returns how many events have been discarded for want of buffer space.
// Non-zero means the recorded stream has holes, which an analysis should know
// before drawing conclusions from event counts.
func (w *Writer) Dropped() int64 {
	if w == nil {
		return 0
	}
	return w.dropped.Load()
}

// Written returns how many events have reached the database.
func (w *Writer) Written() int64 {
	if w == nil {
		return 0
	}
	return w.written.Load()
}

// Close stops accepting events and flushes what is already buffered.
//
// Best effort with a bounded wait: shutdown must not hang on a database that is
// already gone, so a drain that cannot finish is abandoned rather than allowed to
// hold up the process.
func (w *Writer) Close() error {
	if w == nil {
		return nil
	}
	w.stopOnce.Do(func() { close(w.stop) })
	select {
	case <-w.done:
	case <-time.After(5 * time.Second):
	}
	return nil
}

func (w *Writer) run() {
	defer close(w.done)

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	batch := make([]Event, 0, w.batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.writeBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case e := <-w.queue:
			batch = append(batch, e)
			if len(batch) >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-w.stop:
			// Drain what is already queued, then leave. Only what is in hand: new
			// arrivals after Close are not waited for, or shutdown could be held open
			// indefinitely by a caller still recording.
			for {
				select {
				case e := <-w.queue:
					batch = append(batch, e)
					if len(batch) >= w.batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

// writeBatch inserts a batch as one multi-row statement.
//
// Every failure here is swallowed after logging. There is no caller left to return
// an error to -- that is the whole design -- and a retry loop would turn a database
// problem into an unbounded backlog competing with live traffic for connections.
func (w *Writer) writeBatch(batch []Event) {
	if w.db == nil || len(batch) == 0 {
		return
	}

	const columns = 9
	values := make([]string, 0, len(batch))
	args := make([]any, 0, len(batch)*columns)

	for _, e := range batch {
		encoded, err := json.Marshal(e.Payload)
		if err != nil {
			// One unencodable payload must not cost the rest of the batch.
			log.Printf("activity: dropping event %q with unencodable payload: %v", e.Type, err)
			w.dropped.Add(1)
			continue
		}

		n := len(args)
		placeholders := make([]string, columns)
		for i := range placeholders {
			placeholders[i] = "$" + strconv.Itoa(n+i+1)
		}
		values = append(values, "("+strings.Join(placeholders, ", ")+")")

		args = append(args,
			e.Type,
			e.Source,
			uuidArg(e.UserID),
			uuidArg(e.GameID),
			textArg(e.ModeKey),
			uuidArg(e.SessionID),
			e.OccurredAt.UTC(),
			w.now().UTC(),
			encoded,
		)
	}
	if len(values) == 0 {
		return
	}

	// Its own timeout, unrelated to any request context. The request that produced
	// these events finished long ago; inheriting its context would cancel the write
	// for reasons that have nothing to do with the write.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		INSERT INTO player_activity_events
			(event_type, source, user_id, game_id, mode_key, session_id, occurred_at, recorded_at, payload)
		VALUES ` + strings.Join(values, ", ")

	if _, err := w.db.ExecContext(ctx, query, args...); err != nil {
		log.Printf("activity: dropping %d events, insert failed: %v", len(values), err)
		w.dropped.Add(int64(len(values)))
		return
	}
	w.written.Add(int64(len(values)))
}

func uuidArg(id *uuid.UUID) any {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return *id
}

func textArg(s string) any {
	if s == "" {
		return nil
	}
	return s
}
