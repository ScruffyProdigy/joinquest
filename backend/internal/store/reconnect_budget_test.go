package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// The disconnect grace windows in this package are each one half of a decision whose
// other half lives in the frontend, and the rule they share is stated on
// DefaultRoomDisconnectGrace: we hold your place for exactly as long as your client is
// still asking for it, and not one window longer.
//
// Pinning each side separately does not pin that rule. Two tests, one asserting the Go
// constant and one asserting the JS retry count, both stay green while somebody raises
// one of them — which is exactly how the queue pair drifted to a 67s gap (JQ-283).
// So this test reads the real budget out of the frontend source and derives it here,
// which is the only place the relationship can actually be checked.
//
// It couples a Go test to JS source deliberately. The alternative is the drift.

// reconnectCeilingWait is the top of the backoff curve the frontend clients share, and
// therefore the most a budget can be short by while the client is still, in any useful
// sense, asking at the moment the server gives up.
const reconnectCeilingWait = 5 * time.Second

// reconnectBudget is the frontend's curve, re-derived rather than trusted: graphql-ws
// passes retryWait a 0-based count, so attempt i waits min(500ms*i, 5s) and the first
// waits nothing. Reading the count off a 1-based curve is how a previous derivation
// quoted 27.5s for what was really 22.5s.
func reconnectBudget(attempts int) time.Duration {
	var total time.Duration
	for i := 0; i < attempts; i++ {
		wait := time.Duration(500*i) * time.Millisecond
		if wait > reconnectCeilingWait {
			wait = reconnectCeilingWait
		}
		total += wait
	}
	return total
}

// readReconnectAttempts pulls `export const <name> = <n>` out of a frontend lib file.
// A miss fails loudly rather than defaulting, because a silently-zero budget would make
// every assertion below pass for the wrong reason.
func readReconnectAttempts(t *testing.T, file, name string) int {
	t.Helper()
	path := filepath.Join("..", "..", "..", "frontend", "src", "lib", file)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	match := regexp.MustCompile(
		fmt.Sprintf(`export const %s\s*=\s*(\d+)`, regexp.QuoteMeta(name)),
	).FindSubmatch(src)
	if match == nil {
		t.Fatalf("%s does not export %s. If it was renamed, this pairing still has to be"+
			" checked somewhere — update this test rather than deleting it", path, name)
	}
	attempts, err := strconv.Atoi(string(match[1]))
	if err != nil {
		t.Fatalf("%s in %s is not a number: %v", name, path, err)
	}
	// The curve itself has to be the one derived above, or the count means nothing.
	curve := regexp.MustCompile(`Math\.min\(500 \* retries, 5000\)`)
	if !curve.Match(src) {
		t.Fatalf("%s no longer computes its backoff as min(500 * retries, 5000); the"+
			" budget derived in this test does not describe it any more", path)
	}
	return attempts
}

func TestClientReconnectBudgetsMatchTheirGraceWindows(t *testing.T) {
	for _, tc := range []struct {
		what   string
		file   string
		export string
		grace  time.Duration
		why    string
	}{
		{
			what:   "queue place",
			file:   "queue.js",
			export: "QUEUE_RECONNECT_ATTEMPTS",
			grace:  DefaultQueueDisconnectGrace,
			why: "a queue place is rivalrous, so every second it is held for a browser" +
				" that stopped asking is a second the people behind it wait for nothing",
		},
		{
			what:   "forming-table seat",
			file:   "tables.js",
			export: "TABLE_SEAT_RECONNECT_ATTEMPTS",
			grace:  DefaultTableSeatDisconnectGrace,
			why: "a held seat is the one thing another player at a forming table is" +
				" actively waiting on",
		},
		{
			what:   "room membership",
			file:   "rooms.js",
			export: "ROOM_RECONNECT_ATTEMPTS",
			grace:  DefaultRoomDisconnectGrace,
			why: "a room held open for a browser that has given up is not a held place" +
				" but a lie with a longer lifetime",
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			budget := reconnectBudget(readReconnectAttempts(t, tc.file, tc.export))

			// Under, not over: a client still asking after the server has released the
			// place reconnects into nothing, and reads that as having been kicked.
			if budget >= tc.grace {
				t.Fatalf("%s: client keeps asking for %s but the server only holds for %s."+
					" Raise the grace window or lower %s — %s",
					tc.what, budget, tc.grace, tc.export, tc.why)
			}

			// And not far under. This is the half that the separate per-side tests
			// could never catch, and the half the queue pair actually broke.
			if gap := tc.grace - budget; gap >= reconnectCeilingWait {
				t.Fatalf("%s: the server holds for %s but the client gives up after %s,"+
					" leaving %s in which the place is held for a browser that has"+
					" stopped asking for it. Raise %s to close it — %s",
					tc.what, tc.grace, budget, gap, tc.export, tc.why)
			}
		})
	}
}

// The values as they stand, so a change has to be deliberate about the number and not
// only about the invariant above.
func TestReconnectBudgetsAreTheDerivedValues(t *testing.T) {
	for _, tc := range []struct {
		file   string
		export string
		want   time.Duration
	}{
		{"queue.js", "QUEUE_RECONNECT_ATTEMPTS", 87500 * time.Millisecond},
		{"tables.js", "TABLE_SEAT_RECONNECT_ATTEMPTS", 27500 * time.Millisecond},
		{"rooms.js", "ROOM_RECONNECT_ATTEMPTS", 297500 * time.Millisecond},
	} {
		t.Run(tc.export, func(t *testing.T) {
			if got := reconnectBudget(readReconnectAttempts(t, tc.file, tc.export)); got != tc.want {
				t.Fatalf("%s budget: got %s, want %s", tc.export, got, tc.want)
			}
		})
	}
}
