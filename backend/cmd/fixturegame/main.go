// Standalone fixture game server for local dev: serves the per-player endpoints the
// "Eligibility Fixture" catalog game (migration 000038) points at — mode-eligibility
// and queue-options — so the frontend locked/unlocked UI and the pre-queue picker can
// be visually verified without a real third-party game integration.
// Usage: go run ./cmd/fixturegame
package main

import (
	"log"
	"net/http"

	"github.com/scruffyprodigy/joinquest/internal/gameclient/testutil"
)

func main() {
	addr := ":9400"
	mux := http.NewServeMux()
	mux.Handle("/api/v1/players/", routeByEndpoint(
		testutil.EligibilityFixtureHandler(),
		testutil.QueueOptionsFixtureHandler(),
	))
	log.Printf("fixturegame listening on %s (mode-eligibility, queue-options)", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// routeByEndpoint dispatches on the trailing path segment. Both fixtures already
// match on their own suffix, so this just picks which one gets to answer.
func routeByEndpoint(eligibility, queueOptions http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case pathHasSuffix(r.URL.Path, "/queue-options"):
			queueOptions.ServeHTTP(w, r)
		case pathHasSuffix(r.URL.Path, "/mode-eligibility"):
			eligibility.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func pathHasSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}
