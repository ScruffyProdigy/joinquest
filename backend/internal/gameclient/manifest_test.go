package gameclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scruffyprodigy/playhub/internal/seattemplate"
)

func newManifestTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case "/api/v1/status":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"game":"Demo Game","version":"1.2.3","appEnv":"test","standalone":true}`))
		case "/api/v1/game-modes":
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"modes": [{
					"key": "classic",
					"displayName": "Classic",
					"seatTemplate": {"count": 2}
				}]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestManifestFetcherFetch(t *testing.T) {
	srv := newManifestTestServer(t)
	defer srv.Close()

	fetcher := NewManifestFetcher()
	manifest, err := fetcher.Fetch(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(manifest.Modes) != 1 {
		t.Fatalf("expected 1 mode, got %d", len(manifest.Modes))
	}
	if manifest.Modes[0].Key != "classic" {
		t.Fatalf("expected mode key classic, got %q", manifest.Modes[0].Key)
	}
	if manifest.Status.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", manifest.Status.Version)
	}
	if manifest.ETag != `"v1"` {
		t.Fatalf("expected etag %q, got %q", `"v1"`, manifest.ETag)
	}
	if manifest.SHA256Hash == "" {
		t.Fatal("expected non-empty manifest hash")
	}
}

func TestManifestFetcherNotModified(t *testing.T) {
	srv := newManifestTestServer(t)
	defer srv.Close()

	fetcher := NewManifestFetcher()
	_, err := fetcher.Fetch(context.Background(), srv.URL, `"v1"`)
	if err == nil {
		t.Fatal("expected ErrManifestNotModified")
	}
	if err != ErrManifestNotModified {
		t.Fatalf("expected ErrManifestNotModified, got %v", err)
	}
}

func TestValidateModesRejectsEmptySeatTemplate(t *testing.T) {
	err := validateModes([]ModeManifest{{
		Key:         "solo",
		DisplayName: "Solo",
	}})
	if err == nil {
		t.Fatal("expected validation error for mode without seatTemplate")
	}
}

func TestValidateModesRejectsFlatSeats(t *testing.T) {
	err := validateModes([]ModeManifest{{
		Key:          "duel",
		DisplayName:  "Duel",
		SeatTemplate: json.RawMessage(`{"count":2}`),
		Seats:        json.RawMessage(`[{"key":"a"}]`),
	}})
	if err == nil {
		t.Fatal("expected validation error for flat seats[]")
	}
}

func TestModePlayerBoundsDerivesFromPathsWhenNoTopLevelOverride(t *testing.T) {
	raw := json.RawMessage(`{
		"ClueGiver":{"name":["Red","Blue","Green"],"min":2,"max":3,"sizeForQueue":2},
		"Guesser":{"count":6,"min":2,"max":6,"sizeForQueue":4}
	}`)
	specs, err := seattemplate.PathSpecs(raw)
	if err != nil {
		t.Fatal(err)
	}
	mode := ModeManifest{Key: "party", SeatTemplate: raw}
	min, max := ModePlayerBounds(mode, 9, specs)
	if min != 4 || max != 9 {
		t.Fatalf("got (%d, %d), want (4, 9) derived from per-path min/max, no top-level override needed", min, max)
	}
}

func TestModePlayerBoundsExplicitTopLevelOverrideTakesPrecedence(t *testing.T) {
	raw := json.RawMessage(`{
		"ClueGiver":{"name":["Red","Blue","Green"],"min":2,"max":3,"sizeForQueue":2},
		"Guesser":{"count":6,"min":2,"max":6,"sizeForQueue":4}
	}`)
	specs, err := seattemplate.PathSpecs(raw)
	if err != nil {
		t.Fatal(err)
	}
	mode := ModeManifest{Key: "party", SeatTemplate: raw, Min: 3, Max: 8}
	min, max := ModePlayerBounds(mode, 9, specs)
	if min != 3 || max != 8 {
		t.Fatalf("got (%d, %d), want (3, 8) — explicit manifest override should win over path-derived bounds", min, max)
	}
}

func TestModePlayerBoundsFixedTemplateFallsBackToLeafCount(t *testing.T) {
	raw := json.RawMessage(`{"count":2}`)
	specs, err := seattemplate.PathSpecs(raw)
	if err != nil {
		t.Fatal(err)
	}
	mode := ModeManifest{Key: "duel", SeatTemplate: raw}
	min, max := ModePlayerBounds(mode, 2, specs)
	if min != 2 || max != 2 {
		t.Fatalf("got (%d, %d), want (2, 2)", min, max)
	}
}
