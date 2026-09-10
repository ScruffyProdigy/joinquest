// Command roomsweep runs three fleet-wide room sweeps and emits what each found as an
// alertable signal.
//
// The first removes room memberships whose player's last socket closed longer ago than
// store.DefaultRoomDisconnectGrace, and closes whichever rooms that emptied. It is the
// crash backstop for the in-process timer, not the normal path: when the API process
// survives, PresenceTracker removes the member at exactly the window. This catches the
// windows whose timer died with their pod — the same relationship queuesweep's
// disconnected sweep has to the queue's timer.
//
// The second closes open rooms that nothing is left in. It is not a coarser version of
// the first: a room whose players went into a game is held open on purpose while the
// match runs, and by the time it ends there is no member left whose departure would
// close it. Nothing else ever reconsiders those rooms.
//
// The third deletes rooms that have been closed longer than
// store.DefaultClosedRoomRetention. Closing is the removal players see; this is the row
// actually going away, deferred so the regroup path — which reads a table through its
// closed room on purpose — is long finished with it. Without this half, minting a fresh
// room on every Play with friends click (JQ-253) would grow the table forever.
//
// All three always run. A problem in one is reported in the exit status, never by
// skipping the others. See k8s/jobs/room-cleanup.yaml and docs/lobby-maintenance.md.
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
	metricStaleMembers     = "lobby.room.stale_members.count"
	metricMembersRemoved   = "lobby.room.stale_members.removed"
	metricMembersRemains   = "lobby.room.stale_members.remaining"
	metricRoomsClosed      = "lobby.room.closed"
	metricEmptyRoomsClosed = "lobby.room.empty_closed"
	metricEmptyRooms       = "lobby.room.empty.count"
	metricRetiredRooms     = "lobby.room.retired.count"
	metricRetiredsDeleted  = "lobby.room.retired.deleted"
)

func main() {
	var (
		databaseURL = flag.String("database-url", "", "Database connection URL (defaults to $DATABASE_URL)")
		retention   = flag.Duration("retention", 0, "How long a closed room survives before deletion (defaults to $CLOSED_ROOM_RETENTION, then 6h)")
		dryRun      = flag.Bool("dry-run", false, "Report what each sweep would touch without writing")
		timeout     = flag.Duration("timeout", 2*time.Minute, "Overall timeout for the sweep")
	)
	flag.Parse()

	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
		if *databaseURL == "" {
			log.Fatal("Database URL is required. Set DATABASE_URL or use -database-url")
		}
	}

	keepFor, err := resolveRetention(*retention, os.Getenv("CLOSED_ROOM_RETENTION"))
	if err != nil {
		log.Fatalf("Invalid closed room retention: %v", err)
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

	tags := map[string]string{"job": "room-cleanup"}

	if *dryRun {
		// Every count, for the same reason every sweep runs: a failure in one is not a
		// reason to stop reporting the others.
		dryRunFailed := false

		members, err := st.CountStaleDisconnectedRoomMembers(ctx, store.DefaultRoomDisconnectGrace)
		if err != nil {
			log.Printf("Failed to count stale disconnected room members: %v", err)
			dryRunFailed = true
		} else {
			emitter.Gauge(metricStaleMembers, float64(members), tags)
			log.Printf("dry run: %d room memberships older than %s", members, store.DefaultRoomDisconnectGrace)
		}

		empty, err := st.CountEmptyOpenRooms(ctx)
		if err != nil {
			log.Printf("Failed to count empty open rooms: %v", err)
			dryRunFailed = true
		} else {
			emitter.Gauge(metricEmptyRooms, float64(empty), tags)
			log.Printf("dry run: %d open rooms with nobody in them", empty)
		}

		retired, err := st.CountRetiredRooms(ctx, keepFor)
		if err != nil {
			log.Printf("Failed to count retired rooms: %v", err)
			dryRunFailed = true
		} else {
			emitter.Gauge(metricRetiredRooms, float64(retired), tags)
			log.Printf("dry run: %d rooms closed longer than %s", retired, keepFor)
		}

		if dryRunFailed {
			// os.Exit skips deferred cleanup, so honour the emitter's contract here.
			_ = emitter.Close()
			os.Exit(1)
		}
		return
	}

	// No sweep may end the process, because the others still have to run — the lesson
	// queuesweep records: one wedged row silently disabling another sweep for as long as
	// it sits there. Problems are collected and reported in the exit status instead.
	failed := false

	// The membership sweep is the crash backstop for the in-process timer, not a
	// tunable window, so it always runs against DefaultRoomDisconnectGrace. Only the
	// retention is a flag, because that one is a storage policy rather than a
	// behaviour players feel.
	result, err := st.SweepStaleDisconnectedRoomMembers(ctx, store.DefaultRoomDisconnectGrace)
	if err != nil {
		log.Printf("Failed to sweep stale disconnected room members: %v", err)
		failed = true
	} else {
		emitter.Gauge(metricStaleMembers, float64(result.MembersBefore), tags)
		emitter.Count(metricMembersRemoved, float64(result.MembersRemoved), tags)
		emitter.Gauge(metricMembersRemains, float64(result.MembersAfter), tags)
		emitter.Count(metricRoomsClosed, float64(result.RoomsClosed), tags)

		log.Printf("stale room member sweep: found=%d removed=%d remaining=%d rooms_closed=%d window=%s",
			result.MembersBefore, result.MembersRemoved, result.MembersAfter, result.RoomsClosed,
			store.DefaultRoomDisconnectGrace)

		// Rows that survived their own sweep mean the predicate and the write
		// disagree. Fail the job so the CronJob's failure count is itself a signal.
		if result.MembersAfter > 0 {
			log.Printf("%d stale room memberships survived the sweep", result.MembersAfter)
			failed = true
		}
	}

	// Rooms that emptied without anybody leaving them. A room whose players went into
	// a game is held open deliberately, and when that match ends there is no member
	// left to trigger the close — so this pass, not the one above, is what finishes it.
	// It runs whatever the membership sweep did: they reach different rooms.
	emptied, err := st.SweepEmptyRooms(ctx)
	if err != nil {
		log.Printf("Failed to sweep empty rooms: %v", err)
		failed = true
	} else {
		emitter.Count(metricEmptyRoomsClosed, float64(emptied), tags)
		log.Printf("empty room sweep: closed=%d", emptied)
	}

	retiredBefore, err := st.CountRetiredRooms(ctx, keepFor)
	if err != nil {
		log.Printf("Failed to count retired rooms: %v", err)
		failed = true
	} else {
		deleted, err := st.DeleteRetiredRooms(ctx, keepFor)
		if err != nil {
			log.Printf("Failed to delete retired rooms: %v", err)
			failed = true
		} else {
			emitter.Gauge(metricRetiredRooms, float64(retiredBefore), tags)
			emitter.Count(metricRetiredsDeleted, float64(deleted), tags)
			log.Printf("retired room sweep: found=%d deleted=%d retention=%s",
				retiredBefore, deleted, keepFor)
		}
	}

	if failed {
		// os.Exit skips deferred cleanup, so honour the emitter's contract here.
		_ = emitter.Close()
		os.Exit(1)
	}
}

// resolveRetention picks the retention from the flag, then the environment, then the
// shared default, so the CronJob can be retuned without a code change.
func resolveRetention(flagValue time.Duration, envValue string) (time.Duration, error) {
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
	return store.DefaultClosedRoomRetention, nil
}
