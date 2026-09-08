package gameclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newQueueOptionsTestServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/players/player-1/queue-options" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("modeKey"); got != "duel-helpers" {
			t.Errorf("modeKey query = %q, want duel-helpers", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func fetchHelpers(t *testing.T, srv *httptest.Server) (*QueueOptionRoster, error) {
	t.Helper()
	return NewClient().FetchQueueOptions(context.Background(), srv.URL, "player-1", "duel-helpers", "helpers")
}

func TestFetchQueueOptionsReadsGroups(t *testing.T) {
	srv := newQueueOptionsTestServer(t, `{"groups":[{"key":"helpers","choices":[
		{"id":"ferrus","label":"Ferrus","description":"Robot takes 1 mark","locked":false},
		{"id":"rust","label":"Rust","locked":true,"unlockModeKey":"casual",
		 "requirement":{"kind":"leaf","label":"Casual wins","current":3,"target":5}}
	]}]}`, http.StatusOK)
	defer srv.Close()

	roster, err := fetchHelpers(t, srv)
	if err != nil {
		t.Fatalf("FetchQueueOptions: %v", err)
	}
	if len(roster.Groups) != 1 || roster.Groups[0].Key != "helpers" {
		t.Fatalf("groups = %+v, want one group keyed helpers", roster.Groups)
	}
	choices := roster.Groups[0].Choices
	if len(choices) != 2 {
		t.Fatalf("choices = %d, want 2", len(choices))
	}
	if choices[0].ID != "ferrus" || choices[0].Label != "Ferrus" || choices[0].Locked {
		t.Fatalf("choice[0] = %+v, want an unlocked Ferrus", choices[0])
	}
	if !choices[1].Locked || choices[1].Requirement == nil || choices[1].Requirement.Current != 3 {
		t.Fatalf("choice[1] = %+v, want locked Rust with a 3/5 requirement", choices[1])
	}
	if choices[1].UnlockModeKey == nil || *choices[1].UnlockModeKey != "casual" {
		t.Fatalf("choice[1].UnlockModeKey = %v, want casual", choices[1].UnlockModeKey)
	}
}

func TestFetchQueueOptionsAcceptsFlatChoicesShape(t *testing.T) {
	// The shape JQ-148 specifies for RPSLR: no groups wrapper, one implicit roster.
	srv := newQueueOptionsTestServer(t, `{"choices":[
		{"id":"ferrus","label":"Ferrus","locked":false}
	]}`, http.StatusOK)
	defer srv.Close()

	roster, err := fetchHelpers(t, srv)
	if err != nil {
		t.Fatalf("FetchQueueOptions: %v", err)
	}
	if len(roster.Groups) != 1 || roster.Groups[0].Key != "helpers" {
		t.Fatalf("groups = %+v, want the flat list adopted into the declared group", roster.Groups)
	}
	if len(roster.Groups[0].Choices) != 1 || roster.Groups[0].Choices[0].ID != "ferrus" {
		t.Fatalf("choices = %+v, want ferrus", roster.Groups[0].Choices)
	}
}

func TestFetchQueueOptionsAcceptsEmptyRoster(t *testing.T) {
	srv := newQueueOptionsTestServer(t, `{"groups":[]}`, http.StatusOK)
	defer srv.Close()

	roster, err := fetchHelpers(t, srv)
	if err != nil {
		t.Fatalf("FetchQueueOptions: %v", err)
	}
	if len(roster.Groups) != 0 {
		t.Fatalf("groups = %+v, want none", roster.Groups)
	}
}

func TestFetchQueueOptionsFailsClosedOnNotFound(t *testing.T) {
	srv := newQueueOptionsTestServer(t, `nope`, http.StatusNotFound)
	defer srv.Close()

	// Unlike mode-eligibility, there is no safe default roster: guessing empty
	// blocks a legitimate join and guessing "everything" hands out options the
	// game never offered. A missing endpoint has to be visible.
	if _, err := fetchHelpers(t, srv); err == nil {
		t.Fatal("FetchQueueOptions = nil error on 404, want an error")
	}
}

func TestFetchQueueOptionsFailsClosedOnServerError(t *testing.T) {
	srv := newQueueOptionsTestServer(t, `boom`, http.StatusInternalServerError)
	defer srv.Close()

	if _, err := fetchHelpers(t, srv); err == nil {
		t.Fatal("FetchQueueOptions = nil error on 500, want an error")
	}
}

func TestFetchQueueOptionsFailsClosedOnMalformedBody(t *testing.T) {
	srv := newQueueOptionsTestServer(t, `{"groups":`, http.StatusOK)
	defer srv.Close()

	if _, err := fetchHelpers(t, srv); err == nil {
		t.Fatal("FetchQueueOptions = nil error on truncated JSON, want an error")
	}
}

func TestFetchQueueOptionsDropsMalformedRequirementButKeepsTheLock(t *testing.T) {
	srv := newQueueOptionsTestServer(t, `{"groups":[{"key":"helpers","choices":[
		{"id":"rust","label":"Rust","locked":true,"requirement":{"kind":"wat"}}
	]}]}`, http.StatusOK)
	defer srv.Close()

	roster, err := fetchHelpers(t, srv)
	if err != nil {
		t.Fatalf("FetchQueueOptions: %v", err)
	}
	choice := roster.Groups[0].Choices[0]
	if !choice.Locked {
		t.Fatal("choice.Locked = false, want the lock kept when only its progress tree is bad")
	}
	if choice.Requirement != nil {
		t.Fatalf("choice.Requirement = %+v, want the unreadable tree dropped", choice.Requirement)
	}
}

func TestFetchQueueOptionsRequiresModeKey(t *testing.T) {
	srv := newQueueOptionsTestServer(t, `{"groups":[]}`, http.StatusOK)
	defer srv.Close()

	_, err := NewClient().FetchQueueOptions(context.Background(), srv.URL, "player-1", "  ", "helpers")
	if err == nil || !strings.Contains(err.Error(), "mode key") {
		t.Fatalf("err = %v, want a mode key error", err)
	}
}

func TestQueueOptionRosterValidationViewCarriesIDsAndLocks(t *testing.T) {
	roster := QueueOptionRoster{Groups: []QueueOptionGroup{{Key: "helpers", Choices: []QueueOption{
		{ID: "ferrus"},
		{ID: "rust", Locked: true},
	}}}}

	view := roster.ValidationView()

	if len(view) != 1 || view[0].Key != "helpers" || len(view[0].Choices) != 2 {
		t.Fatalf("view = %+v, want one group with two choices", view)
	}
	if view[0].Choices[0].ID != "ferrus" || view[0].Choices[0].Locked {
		t.Fatalf("view choice[0] = %+v, want unlocked ferrus", view[0].Choices[0])
	}
	if !view[0].Choices[1].Locked {
		t.Fatalf("view choice[1] = %+v, want rust locked", view[0].Choices[1])
	}
}
