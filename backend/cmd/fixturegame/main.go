// Standalone fixture game server for local dev: serves the mode-eligibility endpoint the
// "Eligibility Fixture" catalog game (migration 000038) points at, so the frontend locked/
// unlocked UI can be visually verified without a real third-party game integration.
// Usage: go run ./cmd/fixturegame
package main

import (
	"log"
	"net/http"

	"github.com/scruffyprodigy/playhub/internal/gameclient/testutil"
)

func main() {
	addr := ":9400"
	log.Printf("fixturegame listening on %s (mode-eligibility endpoint)", addr)
	if err := http.ListenAndServe(addr, testutil.EligibilityFixtureHandler()); err != nil {
		log.Fatal(err)
	}
}
