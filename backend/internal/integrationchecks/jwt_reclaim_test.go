package integrationchecks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// seatPolicy is how a game answers a claim on a seat that is already held.
type seatPolicy int

const (
	// policyReClaim compares sub, the behaviour the protocol asks for (JQ-86).
	policyReClaim seatPolicy = iota
	// policyFlat409 refuses every second claim. This is the bug the check exists
	// to catch: it keeps thieves out and locks real players out with them.
	policyFlat409
	// policyOpenSeat lets anyone claim a held seat — seat theft.
	policyOpenSeat
)

// newSeatClaimServer simulates a game's claim endpoint, tracking who holds each
// seat so a second claim can be answered according to policy.
func newSeatClaimServer(t *testing.T, policy seatPolicy) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	holders := map[string]string{}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims := jwt.MapClaims{}
		if _, _, err := jwt.NewParser().ParseUnverified(raw, claims); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		sub, _ := claims["sub"].(string)
		seat, _ := claims["seatKey"].(string)

		mu.Lock()
		defer mu.Unlock()
		holder, taken := holders[seat]
		switch {
		case !taken:
			holders[seat] = sub
			w.WriteHeader(http.StatusOK)
		case policy == policyOpenSeat:
			holders[seat] = sub
			w.WriteHeader(http.StatusOK)
		case policy == policyFlat409:
			w.WriteHeader(http.StatusConflict)
		case holder == sub:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusConflict)
		}
	}))
}

func TestReClaimChecksPassWhenGameComparesSub(t *testing.T) {
	signer, _ := testSignerWithJWKS(t)
	gameSrv := newSeatClaimServer(t, policyReClaim)
	t.Cleanup(gameSrv.Close)

	results := (&Runner{}).RunJWTChecks(context.Background(), testGame(gameSrv.URL), signer, "match-1", "seat-a", "")

	if got := resultStatus(t, results, "jwt.reclaim_same_player"); got != StatusPass {
		t.Fatalf("jwt.reclaim_same_player = %s, want pass", got)
	}
	if got := resultStatus(t, results, "jwt.reclaim_seat_theft"); got != StatusPass {
		t.Fatalf("jwt.reclaim_seat_theft = %s, want pass", got)
	}
}

// The whole reason the check is split in two: a flat 409 looks like correct,
// defensive behaviour and passes the theft half, while locking a player out of
// the match they are still seated in.
func TestReClaimChecksCatchFlat409(t *testing.T) {
	signer, _ := testSignerWithJWKS(t)
	gameSrv := newSeatClaimServer(t, policyFlat409)
	t.Cleanup(gameSrv.Close)

	results := (&Runner{}).RunJWTChecks(context.Background(), testGame(gameSrv.URL), signer, "match-1", "seat-a", "")

	if got := resultStatus(t, results, "jwt.reclaim_same_player"); got != StatusFail {
		t.Fatalf("jwt.reclaim_same_player = %s, want fail — a flat 409 locks a returning player out", got)
	}
	if got := resultStatus(t, results, "jwt.reclaim_seat_theft"); got != StatusPass {
		t.Fatalf("jwt.reclaim_seat_theft = %s, want pass — a flat 409 does keep thieves out", got)
	}
}

func TestReClaimChecksCatchSeatTheft(t *testing.T) {
	signer, _ := testSignerWithJWKS(t)
	gameSrv := newSeatClaimServer(t, policyOpenSeat)
	t.Cleanup(gameSrv.Close)

	results := (&Runner{}).RunJWTChecks(context.Background(), testGame(gameSrv.URL), signer, "match-1", "seat-a", "")

	if got := resultStatus(t, results, "jwt.reclaim_same_player"); got != StatusPass {
		t.Fatalf("jwt.reclaim_same_player = %s, want pass", got)
	}
	if got := resultStatus(t, results, "jwt.reclaim_seat_theft"); got != StatusFail {
		t.Fatalf("jwt.reclaim_seat_theft = %s, want fail — anyone can take an occupied seat", got)
	}
}

// Provision never ran, so there is no seat to re-claim; the rows must still be
// present and skipped rather than missing from the report.
func TestReClaimChecksSkippedWithoutProvision(t *testing.T) {
	results := JWTSkippedNoProvision()
	if got := resultStatus(t, results, "jwt.reclaim_same_player"); got != StatusSkipped {
		t.Fatalf("jwt.reclaim_same_player = %s, want skipped", got)
	}
	if got := resultStatus(t, results, "jwt.reclaim_seat_theft"); got != StatusSkipped {
		t.Fatalf("jwt.reclaim_seat_theft = %s, want skipped", got)
	}
}
