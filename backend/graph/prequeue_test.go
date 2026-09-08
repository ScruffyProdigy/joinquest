package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

const rosterBody = `{"groups":[{"key":"helpers","choices":[
	{"id":"ferrus","label":"Ferrus","locked":false},
	{"id":"tempered","label":"Tempered","locked":false},
	{"id":"rust","label":"Rust","locked":true,
	 "requirement":{"kind":"leaf","label":"Casual wins","current":3,"target":5}}
]}]}`

func newRosterResolver(t *testing.T, body string, status int) (*Resolver, *store.Game, *store.GameMode) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	apiBaseURL := srv.URL
	game := &store.Game{ID: uuid.New(), APIBaseURL: &apiBaseURL}
	mode := &store.GameMode{
		ID:      uuid.New(),
		GameID:  game.ID,
		ModeKey: "duel-helpers",
		PreQueue: json.RawMessage(`{"groups":[
			{"key":"helpers","kind":"Loadout","label":"Choose your two helpers","min":2,"max":2}
		]}`),
	}
	r := &Resolver{QueueOptionsCache: gameclient.NewQueueOptionsCache(gameclient.NewClient(), time.Second)}
	return r, game, mode
}

func picks(ids ...string) []*model.QueueOptionSelectionInput {
	return []*model.QueueOptionSelectionInput{{GroupKey: "helpers", OptionIds: ids}}
}

func TestResolveSelectionsAcceptsAValidPickAndStampsLabels(t *testing.T) {
	r, game, mode := newRosterResolver(t, rosterBody, http.StatusOK)

	got, err := r.resolveSelections(context.Background(), game, mode, "player-1", picks("tempered", "ferrus"))
	if err != nil {
		t.Fatalf("resolveSelections: %v", err)
	}
	if len(got) != 1 || len(got[0].OptionIDs) != 2 {
		t.Fatalf("selections = %+v, want one group with two ids", got)
	}
	if got[0].Labels[0] != "Ferrus" || got[0].Labels[1] != "Tempered" {
		t.Fatalf("labels = %v, want the game's own names", got[0].Labels)
	}
}

func TestResolveSelectionsRejectsALockedOption(t *testing.T) {
	r, game, mode := newRosterResolver(t, rosterBody, http.StatusOK)

	// The UI disables this card, but a client can send anything; the lock has to
	// hold on the server.
	_, err := r.resolveSelections(context.Background(), game, mode, "player-1", picks("ferrus", "rust"))
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("err = %v, want a locked-option rejection", err)
	}
}

func TestResolveSelectionsRejectsAnOptionTheGameNeverOffered(t *testing.T) {
	r, game, mode := newRosterResolver(t, rosterBody, http.StatusOK)

	_, err := r.resolveSelections(context.Background(), game, mode, "player-1", picks("ferrus", "godmode"))
	if err == nil || !strings.Contains(err.Error(), "godmode") {
		t.Fatalf("err = %v, want the invented option named and rejected", err)
	}
}

func TestResolveSelectionsRejectsTheWrongNumberOfPicks(t *testing.T) {
	r, game, mode := newRosterResolver(t, rosterBody, http.StatusOK)

	if _, err := r.resolveSelections(context.Background(), game, mode, "player-1", picks("ferrus")); err == nil {
		t.Fatal("resolveSelections = nil error for one pick in a 2..2 group")
	}
}

func TestResolveSelectionsRequiresPicksWhenTheModeDeclaresAGroup(t *testing.T) {
	r, game, mode := newRosterResolver(t, rosterBody, http.StatusOK)

	_, err := r.resolveSelections(context.Background(), game, mode, "player-1", nil)
	if err == nil || !strings.Contains(err.Error(), "helpers") {
		t.Fatalf("err = %v, want the missing group named", err)
	}
}

func TestResolveSelectionsBlocksTheJoinWhenTheGameCannotAnswer(t *testing.T) {
	r, game, mode := newRosterResolver(t, "boom", http.StatusInternalServerError)

	// No safe default exists, so the join fails loudly rather than queueing the
	// player with picks nobody verified.
	_, err := r.resolveSelections(context.Background(), game, mode, "player-1", picks("ferrus", "tempered"))
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("err = %v, want an unavailable-options rejection", err)
	}
}

func TestResolveSelectionsIgnoresTheGameWhenTheModeHasNoGroups(t *testing.T) {
	r, game, mode := newRosterResolver(t, rosterBody, http.StatusOK)
	mode.PreQueue = nil

	got, err := r.resolveSelections(context.Background(), game, mode, "player-1", nil)
	if err != nil {
		t.Fatalf("resolveSelections: %v", err)
	}
	if got != nil {
		t.Fatalf("selections = %+v, want none for a mode without options", got)
	}
}

func TestResolveSelectionsRejectsPicksForAModeWithNoGroups(t *testing.T) {
	r, game, mode := newRosterResolver(t, rosterBody, http.StatusOK)
	mode.PreQueue = nil

	_, err := r.resolveSelections(context.Background(), game, mode, "player-1", picks("ferrus"))
	if err == nil || !strings.Contains(err.Error(), "does not use pre-queue options") {
		t.Fatalf("err = %v, want a rejection for a mode that has no picker", err)
	}
}
