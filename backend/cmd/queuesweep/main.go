// Command queuesweep cancels stale `matched` game_queues rows across all users and
// emits the stale count as an alertable signal.
//
// It is the scheduled, fleet-wide counterpart to the per-user heal that runs lazily
// inside GetUserActiveIntent. See k8s/jobs/stale-matched-queue-sweep.yaml and
// docs/lobby-maintenance.md.
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
	metricStaleBefore = "lobby.queue.stale_matched.count"
	metricCancelled   = "lobby.queue.stale_matched.cancelled"
	metricRemaining   = "lobby.queue.stale_matched.remaining"
)

func main() {
	var (
		databaseURL = flag.String("database-url", "", "Database connection URL (defaults to $DATABASE_URL)")
		olderThan   = flag.Duration("older-than", 0, "Age a matched row must exceed to count as stale (defaults to $STALE_MATCHED_QUEUE_AGE, then 5m)")
		dryRun      = flag.Bool("dry-run", false, "Report the stale count without cancelling anything")
		timeout     = flag.Duration("timeout", 2*time.Minute, "Overall timeout for the sweep")
	)
	flag.Parse()

	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
		if *databaseURL == "" {
			log.Fatal("Database URL is required. Set DATABASE_URL or use -database-url")
		}
	}

	threshold, err := resolveThreshold(*olderThan, os.Getenv("STALE_MATCHED_QUEUE_AGE"))
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

	tags := map[string]string{"job": "stale-matched-queue-sweep"}

	if *dryRun {
		count, err := st.CountStaleMatchedQueues(ctx, threshold)
		if err != nil {
			log.Fatalf("Failed to count stale matched queues: %v", err)
		}
		emitter.Gauge(metricStaleBefore, float64(count), tags)
		log.Printf("dry run: %d stale matched queue rows older than %s", count, threshold)
		return
	}

	result, err := st.SweepStaleMatchedQueues(ctx, threshold)
	if err != nil {
		log.Fatalf("Failed to sweep stale matched queues: %v", err)
	}

	emitter.Gauge(metricStaleBefore, float64(result.StaleBefore), tags)
	emitter.Count(metricCancelled, float64(result.Cancelled), tags)
	emitter.Gauge(metricRemaining, float64(result.StaleAfter), tags)

	log.Printf("stale matched queue sweep: found=%d cancelled=%d remaining=%d threshold=%s",
		result.StaleBefore, result.Cancelled, result.StaleAfter, threshold)

	// Rows that survived their own sweep mean the predicate and the write disagree.
	// Fail the job so the CronJob's failure count is itself a signal.
	if result.StaleAfter > 0 {
		log.Fatalf("%d stale matched queue rows survived the sweep", result.StaleAfter)
	}
}

// resolveThreshold picks the staleness threshold from the flag, then the environment,
// then the shared default, so the CronJob can be retuned without a code change.
func resolveThreshold(flagValue time.Duration, envValue string) (time.Duration, error) {
	if flagValue > 0 {
		return flagValue, nil
	}
	if envValue != "" {
		parsed, err := time.ParseDuration(envValue)
		if err != nil {
			return 0, err
		}
		if parsed > 0 {
			return parsed, nil
		}
	}
	return store.DefaultStaleMatchedQueueAge, nil
}
