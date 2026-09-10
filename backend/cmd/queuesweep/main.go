// Command queuesweep runs two fleet-wide queue sweeps and emits what each found as an
// alertable signal.
//
// The first cancels stale `matched` rows — the scheduled counterpart to the per-user
// heal that runs lazily inside GetUserActiveIntent, which only ever fires for whoever
// happens to make a request.
//
// The second cancels `waiting` rows whose player's last socket closed longer ago than
// the grace window. That one removes a player who is still nominally queued, so it is
// player-visible in a way the matched sweep is not. It is a backstop, not the normal
// path: when the API process survives, an in-process timer removes the player at
// exactly 90s. This catches the windows whose timer died with its pod.
//
// Both sweeps always run. A problem in one is reported in the exit status, never by
// skipping the other. See k8s/jobs/stale-matched-queue-sweep.yaml and
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

	metricStaleDisconnected     = "lobby.queue.stale_disconnected.count"
	metricDisconnectedCancelled = "lobby.queue.stale_disconnected.cancelled"
	metricDisconnectedRemaining = "lobby.queue.stale_disconnected.remaining"
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
		// Both counts, for the same reason both sweeps run: a failure in one is not a
		// reason to stop reporting the other.
		dryRunFailed := false

		count, err := st.CountStaleMatchedQueues(ctx, threshold)
		if err != nil {
			log.Printf("Failed to count stale matched queues: %v", err)
			dryRunFailed = true
		} else {
			emitter.Gauge(metricStaleBefore, float64(count), tags)
			log.Printf("dry run: %d stale matched queue rows older than %s", count, threshold)
		}

		disconnectedCount, err := st.CountStaleDisconnectedQueues(ctx, store.DefaultQueueDisconnectGrace)
		if err != nil {
			log.Printf("Failed to count stale disconnected queues: %v", err)
			dryRunFailed = true
		} else {
			emitter.Gauge(metricStaleDisconnected, float64(disconnectedCount), tags)
			log.Printf("dry run: %d stale disconnected queue rows older than %s", disconnectedCount, store.DefaultQueueDisconnectGrace)
		}

		if dryRunFailed {
			// os.Exit skips deferred cleanup, so honour the emitter's contract here.
			_ = emitter.Close()
			os.Exit(1)
		}
		return
	}

	// Neither sweep may end the process, because the other still has to run. A stuck
	// matched row used to abort here, which silently disabled the disconnect backstop
	// for as long as it sat there — one wedged row costing every disconnected player
	// their crash-safety net. Problems are collected and reported in the exit status
	// instead.
	failed := false

	result, err := st.SweepStaleMatchedQueues(ctx, threshold)
	if err != nil {
		log.Printf("Failed to sweep stale matched queues: %v", err)
		failed = true
	} else {
		emitter.Gauge(metricStaleBefore, float64(result.StaleBefore), tags)
		emitter.Count(metricCancelled, float64(result.Cancelled), tags)
		emitter.Gauge(metricRemaining, float64(result.StaleAfter), tags)

		log.Printf("stale matched queue sweep: found=%d cancelled=%d remaining=%d threshold=%s",
			result.StaleBefore, result.Cancelled, result.StaleAfter, threshold)

		// Rows that survived their own sweep mean the predicate and the write
		// disagree. Fail the job so the CronJob's failure count is itself a signal.
		if result.StaleAfter > 0 {
			log.Printf("%d stale matched queue rows survived the sweep", result.StaleAfter)
			failed = true
		}
	}

	// The disconnected sweep is the crash backstop for the in-process grace timer, not
	// a tunable window like the matched sweep above — so it always runs against
	// DefaultQueueDisconnectGrace, never -older-than.
	disconnectedResult, err := st.SweepStaleDisconnectedQueues(ctx, store.DefaultQueueDisconnectGrace)
	if err != nil {
		log.Printf("Failed to sweep stale disconnected queues: %v", err)
		failed = true
	} else {
		emitter.Gauge(metricStaleDisconnected, float64(disconnectedResult.StaleBefore), tags)
		emitter.Count(metricDisconnectedCancelled, float64(disconnectedResult.Cancelled), tags)
		emitter.Gauge(metricDisconnectedRemaining, float64(disconnectedResult.StaleAfter), tags)

		log.Printf("stale disconnected queue sweep: found=%d cancelled=%d remaining=%d threshold=%s",
			disconnectedResult.StaleBefore, disconnectedResult.Cancelled, disconnectedResult.StaleAfter, store.DefaultQueueDisconnectGrace)

		if disconnectedResult.StaleAfter > 0 {
			log.Printf("%d stale disconnected queue rows survived the sweep", disconnectedResult.StaleAfter)
			failed = true
		}
	}

	if failed {
		// os.Exit skips deferred cleanup, so honour the emitter's contract here.
		_ = emitter.Close()
		os.Exit(1)
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
