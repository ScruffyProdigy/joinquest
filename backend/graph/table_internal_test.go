package graph

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func wordHuntModeAndSeats(t *testing.T) (*store.GameMode, []store.GameModeSeat) {
	t.Helper()
	template := json.RawMessage(`{
		"ClueGiver":{"displayName":"Clue Giver","name":["Red","Blue","Green"],"min":2,"max":3,"sizeForQueue":2},
		"Guesser":{"count":6,"min":2,"max":6,"sizeForQueue":4}
	}`)
	mode := &store.GameMode{
		MinPlayers:   4,
		MaxPlayers:   9,
		SeatTemplate: template,
	}
	seats := []store.GameModeSeat{
		{SeatKey: "ClueGiver-Red", QueuePath: strPtr("ClueGiver")},
		{SeatKey: "ClueGiver-Blue", QueuePath: strPtr("ClueGiver")},
		{SeatKey: "ClueGiver-Green", QueuePath: strPtr("ClueGiver")},
		{SeatKey: "Guesser-1", QueuePath: strPtr("Guesser")},
		{SeatKey: "Guesser-2", QueuePath: strPtr("Guesser")},
		{SeatKey: "Guesser-3", QueuePath: strPtr("Guesser")},
		{SeatKey: "Guesser-4", QueuePath: strPtr("Guesser")},
		{SeatKey: "Guesser-5", QueuePath: strPtr("Guesser")},
		{SeatKey: "Guesser-6", QueuePath: strPtr("Guesser")},
	}
	return mode, seats
}

func TestTableLookForGroupVisibleWordHuntMinimumRoster(t *testing.T) {
	t.Parallel()
	mode, modeSeats := wordHuntModeAndSeats(t)
	seated := []store.TableSeat{
		{UserID: uuid.New(), SeatKey: "ClueGiver-Red"},
		{UserID: uuid.New(), SeatKey: "ClueGiver-Blue"},
		{UserID: uuid.New(), SeatKey: "Guesser-1"},
		{UserID: uuid.New(), SeatKey: "Guesser-2"},
	}

	visible, err := tableLookForGroupVisible(mode, modeSeats, seated)
	if err != nil {
		t.Fatalf("tableLookForGroupVisible: %v", err)
	}
	if !visible {
		t.Fatal("Look for group should stay visible when minimum roster is met but seats remain")
	}
}

func TestTableLookForGroupVisibleWordHuntFullTable(t *testing.T) {
	t.Parallel()
	mode, modeSeats := wordHuntModeAndSeats(t)
	seated := make([]store.TableSeat, 0, 9)
	for _, key := range []string{
		"ClueGiver-Red", "ClueGiver-Blue", "ClueGiver-Green",
		"Guesser-1", "Guesser-2", "Guesser-3", "Guesser-4", "Guesser-5", "Guesser-6",
	} {
		seated = append(seated, store.TableSeat{UserID: uuid.New(), SeatKey: key})
	}

	visible, err := tableLookForGroupVisible(mode, modeSeats, seated)
	if err != nil {
		t.Fatalf("tableLookForGroupVisible: %v", err)
	}
	if visible {
		t.Fatal("Look for group should be hidden when every role is full")
	}
}

func TestTableLookForGroupVisibleEmptyTable(t *testing.T) {
	t.Parallel()
	mode, modeSeats := wordHuntModeAndSeats(t)

	visible, err := tableLookForGroupVisible(mode, modeSeats, nil)
	if err != nil {
		t.Fatalf("tableLookForGroupVisible: %v", err)
	}
	if visible {
		t.Fatal("Look for group should be hidden with no seated players")
	}
}

func TestSeatDisplayName(t *testing.T) {
	t.Parallel()
	template := json.RawMessage(`{
		"ClueGiver":{"displayName":"Clue Giver","name":["Red","Blue","Green"],"min":2,"max":3,"sizeForQueue":2},
		"Guesser":{"count":2,"min":2,"max":6,"sizeForQueue":4}
	}`)
	seats := []store.GameModeSeat{
		{SeatKey: "ClueGiver-Red", QueuePath: strPtr("ClueGiver")},
		{SeatKey: "Guesser-1", QueuePath: strPtr("Guesser")},
		{SeatKey: "1"},
	}

	if got := seatDisplayName(seats, "ClueGiver-Red", template); got != "Clue Giver · Red" {
		t.Fatalf("ClueGiver-Red = %q, want Clue Giver · Red", got)
	}
	if got := seatDisplayName(seats, "Guesser-1", template); got != "Guesser · 1" {
		t.Fatalf("Guesser-1 = %q, want Guesser · 1", got)
	}
	if got := seatDisplayName(seats, "1", template); got != "1" {
		t.Fatalf("fifo seat = %q, want 1", got)
	}
}

func strPtr(s string) *string {
	return &s
}
