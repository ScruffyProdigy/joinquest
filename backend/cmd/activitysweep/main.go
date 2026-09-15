// Command activitysweep enforces the retention window on JQ-143's player activity
// events, and preserves the answers before the rows behind them go.
//
// Order is the whole job, and it is not an implementation detail: roll up the daily
// funnel, refresh the per-player first-match summary, and only then delete raw events
// past the window. Deleting first would destroy data that is not recoverable from
// anywhere, which is the specific risk the ticket calls out -- longitudinal analysis
// gets wanted at a point when volume does not yet justify having kept everything, and
// deleted is deleted.
//
// It follows that a failure in either rollup ABORTS the deletion rather than being
// reported and stepped over. This is the opposite of how queuesweep and roomsweep
// treat their independent sweeps, and the difference is that their steps are
// independent while these are strictly ordered: raw rows must not be dropped on a run
// that failed to fold them up first. Keeping data too long is a storage cost; dropping
// it early is permanent.
//
// See k8s/jobs/player-activity-sweep.yaml and docs/player-activity-events.md.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/observe"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

const (
	metricRolledUp      = "lobby.activity.daily_rollup.rows"
	metricFirstMatches  = "lobby.activity.first_match_summary.rows"
	metricExpiredBefore = "lobby.activity.expired.count"
	metricDeleted       = "lobby.activity.expired.deleted"
	metricRemaining     = "lobby.activity.expired.remaining"
)

// deleteBatchSize bounds one DELETE. The first run after the window is first reached
// may face a very large backlog, and a single unbounded statement would hold a long
// transaction against a table that live traffic is inserting into.
const deleteBatchSize = 10000

func main() {
	var (
		databaseURL = flag.String("database-url", "", "Database connection URL (defaults to $DATABASE_URL)")
		retention   = flag.Duration("retention", 0, "How long raw activity events are kept (defaults to $ACTIVITY_EVENT_RETENTION, then 180 days)")
		dryRun      = flag.Bool("dry-run", false, "Report what would be rolled up and deleted without writing")
		timeout     = flag.Duration("timeout", 10*time.Minute, "Overall timeout for the sweep")
	)
	flag.Parse()

	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
		if *databaseURL == "" {
			log.Fatal("Database URL is required. Set DATABASE_URL or use -database-url")
		}
	}

	keepFor := *retention
	if keepFor == 0 {
		if raw := os.Getenv("ACTIVITY_EVENT_RETENTION"); raw != "" {
			parsed, err := time.ParseDuration(raw)
			if err != nil {
				log.Fatalf("Invalid ACTIVITY_EVENT_RETENTION %q: %v", raw, err)
			}
			keepFor = parsed
		}
	}
	if keepFor == 0 {
		keepFor = store.DefaultActivityRetention
	}
	if keepFor < 0 {
		log.Fatalf("Retention must not be negative, got %s", keepFor)
	}

	db, err := sql.Open("postgres", *databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	st := store.New(db)
	emitter := observe.NewLogEmitter()
	defer emitter.Close()

	tags := map[string]string{"job": "player-activity-sweep"}
	cutoff := time.Now().Add(-keepFor)

	if *dryRun {
		expired, err := st.CountActivityEventsOlderThan(ctx, cutoff)
		if err != nil {
			log.Fatalf("Failed to count expired activity events: %v", err)
		}
		emitter.Gauge(metricExpiredBefore, float64(expired), tags)
		log.Printf("dry run: %d raw events older than %s (cutoff %s) would be deleted after rollup",
			expired, keepFor, cutoff.UTC().Format(time.RFC3339))
		return
	}

	// Roll up through yesterday. Today is still accumulating, and storing a partial
	// count for it would have to be corrected by a later run -- harmless for the
	// aggregate, misleading for anyone who reads it in between.
	through := time.Now().UTC().AddDate(0, 0, -1)

	rolled, err := st.RollUpActivityDaily(ctx, through)
	if err != nil {
		log.Fatalf("Failed to roll up daily activity; deleting nothing: %v", err)
	}
	emitter.Count(metricRolledUp, float64(rolled), tags)
	log.Printf("rolled up %d daily activity rows through %s", rolled, through.Format("2006-01-02"))

	summarised, err := st.RefreshFirstMatchSummary(ctx)
	if err != nil {
		log.Fatalf("Failed to refresh the first-match summary; deleting nothing: %v", err)
	}
	emitter.Count(metricFirstMatches, float64(summarised), tags)
	log.Printf("refreshed %d first-match summary rows", summarised)

	expired, err := st.CountActivityEventsOlderThan(ctx, cutoff)
	if err != nil {
		log.Fatalf("Failed to count expired activity events: %v", err)
	}
	emitter.Gauge(metricExpiredBefore, float64(expired), tags)

	var deleted int64
	for {
		batch, more, err := st.DeleteActivityEventsOlderThan(ctx, cutoff, deleteBatchSize)
		deleted += batch
		if err != nil {
			// Partial progress is reported rather than discarded: the batches that did
			// land are real, and the next run picks up the rest.
			emitter.Count(metricDeleted, float64(deleted), tags)
			log.Fatalf("Failed to delete expired activity events after %d rows: %v", deleted, err)
		}
		if !more {
			break
		}
		if ctx.Err() != nil {
			log.Printf("timeout reached after deleting %d rows; the next run continues", deleted)
			break
		}
	}

	emitter.Count(metricDeleted, float64(deleted), tags)

	remaining, err := st.CountActivityEventsOlderThan(ctx, cutoff)
	if err != nil {
		log.Printf("Deleted %d expired activity events; failed to recount: %v", deleted, err)
		return
	}
	emitter.Gauge(metricRemaining, float64(remaining), tags)
	log.Printf("deleted %d raw events older than %s; %d remain", deleted, keepFor, remaining)
}
