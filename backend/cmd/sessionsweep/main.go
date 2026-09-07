// Command sessionsweep completes game sessions that have been `active` far longer than
// any real match runs, and emits the stuck count as an alertable signal.
//
// It replaces a raw-psql CronJob that keyed on mode_queue_id — completing every session
// but the newest on a queue — and so ended live games and orphaned their players'
// `matched` queue rows (JQ-171). Age is now the only test, and completion goes through
// store.CompleteSession so a swept session gets the whole treatment rather than a
// status-only UPDATE.
//
// See k8s/jobs/stale-session-cleanup.yaml and docs/lobby-maintenance.md.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/observe"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

const (
	metricStaleBefore = "lobby.session.stale_active.count"
	metricCompleted   = "lobby.session.stale_active.completed"
	metricRemaining   = "lobby.session.stale_active.remaining"
)

func main() {
	var (
		databaseURL = flag.String("database-url", "", "Database connection URL (defaults to $DATABASE_URL)")
		olderThan   = flag.Duration("older-than", 0, "How long a session may stay active before it counts as stuck (defaults to $STALE_SESSION_AGE, then 6h)")
		dryRun      = flag.Bool("dry-run", false, "Report the stuck count without completing anything")
		timeout     = flag.Duration("timeout", 5*time.Minute, "Overall timeout for the sweep")
	)
	flag.Parse()

	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
		if *databaseURL == "" {
			log.Fatal("Database URL is required. Set DATABASE_URL or use -database-url")
		}
	}

	threshold, err := resolveThreshold(*olderThan, os.Getenv("STALE_SESSION_AGE"))
	if err != nil {
		log.Fatalf("Invalid staleness threshold: %v", err)
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

	tags := map[string]string{"job": "stale-session-cleanup"}

	if *dryRun {
		count, err := st.CountStaleSessions(ctx, threshold)
		if err != nil {
			log.Fatalf("Failed to count stale sessions: %v", err)
		}
		emitter.Gauge(metricStaleBefore, float64(count), tags)
		log.Printf("dry run: %d sessions active longer than %s", count, threshold)
		return
	}

	result, err := st.SweepStaleSessions(ctx, threshold)
	if err != nil {
		log.Fatalf("Failed to sweep stale sessions: %v", err)
	}

	emitter.Gauge(metricStaleBefore, float64(result.StaleBefore), tags)
	emitter.Count(metricCompleted, float64(result.Completed), tags)
	emitter.Gauge(metricRemaining, float64(result.StaleAfter), tags)

	log.Printf("stale session sweep: found=%d completed=%d remaining=%d threshold=%s",
		result.StaleBefore, result.Completed, result.StaleAfter, threshold)

	// Sessions that survived their own sweep mean the predicate and the write disagree.
	// Fail the job so the CronJob's failure count is itself a signal.
	if result.StaleAfter > 0 {
		log.Fatalf("%d stale sessions survived the sweep", result.StaleAfter)
	}
}

// resolveThreshold picks the staleness threshold from the flag, then the environment,
// then the shared default, so the CronJob can be retuned without a code change.
//
// A non-positive value is rejected rather than clamped: it would make every active
// session stale, which is the exact failure this job was fixed to stop causing.
func resolveThreshold(flagValue time.Duration, envValue string) (time.Duration, error) {
	if flagValue < 0 {
		return 0, errors.New("threshold must be positive")
	}
	if flagValue > 0 {
		return flagValue, nil
	}
	if envValue != "" {
		parsed, err := time.ParseDuration(envValue)
		if err != nil {
			return 0, err
		}
		if parsed < 0 {
			return 0, errors.New("threshold must be positive")
		}
		if parsed > 0 {
			return parsed, nil
		}
	}
	return store.DefaultStaleSessionAge, nil
}
