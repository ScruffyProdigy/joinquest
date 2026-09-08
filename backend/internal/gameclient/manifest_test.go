package gameclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/developer"
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

func TestParseGameModesReadsTypicalMinutes(t *testing.T) {
	modes, err := parseGameModes([]byte(`{
		"modes": [
			{"key": "arena", "displayName": "Arena", "typicalMinutes": 12, "seatTemplate": {"count": 8}},
			{"key": "duel", "displayName": "Duel", "seatTemplate": {"count": 2}}
		]
	}`))
	if err != nil {
		t.Fatalf("parseGameModes failed: %v", err)
	}
	if modes[0].TypicalMinutes == nil || *modes[0].TypicalMinutes != 12 {
		t.Fatalf("expected arena to declare 12 minutes, got %v", modes[0].TypicalMinutes)
	}
	// A mode that omits the field stays nil rather than becoming zero, so the
	// store can tell "not declared" from "declared as nothing".
	if modes[1].TypicalMinutes != nil {
		t.Fatalf("expected duel to carry no duration, got %v", *modes[1].TypicalMinutes)
	}
}

func TestValidateModesRejectsOutOfRangeTypicalMinutes(t *testing.T) {
	tooLong := developer.MaxTypicalMinutes + 1
	err := validateModes([]ModeManifest{{
		Key:            "arena",
		DisplayName:    "Arena",
		TypicalMinutes: &tooLong,
		SeatTemplate:   json.RawMessage(`{"count":8}`),
	}})
	if err == nil {
		t.Fatal("expected validation error for a duration outside the allowed range")
	}
}

func TestValidateModesAllowsAbsentTypicalMinutes(t *testing.T) {
	err := validateModes([]ModeManifest{{
		Key:          "arena",
		DisplayName:  "Arena",
		SeatTemplate: json.RawMessage(`{"count":8}`),
	}})
	if err != nil {
		t.Fatalf("expected a mode without a duration to validate: %v", err)
	}
}
