package lfg

import (
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/seattemplate"
)

func TestComputePathGaps_WordHuntPartial(t *testing.T) {
	specs := []seattemplate.PathSpec{
		{QueuePath: "ClueGiver", DisplayName: "Clue Giver", SizeForQueue: 2},
		{QueuePath: "Guesser", DisplayName: "Guesser", SizeForQueue: 4},
	}
	assignments := []Assignment{
		{SeatKey: "ClueGiver-Red", UserID: "u1", QueuePath: "ClueGiver"},
		{SeatKey: "Guesser-1", UserID: "u2", QueuePath: "Guesser"},
	}

	gaps := ComputePathGaps(specs, assignments)
	if len(gaps) != 2 {
		t.Fatalf("got %d gaps", len(gaps))
	}
	if gaps[0].Assigned != 1 || gaps[0].Needed != 1 {
		t.Fatalf("ClueGiver gap = %+v, want assigned 1 need 1", gaps[0])
	}
	if gaps[1].Assigned != 1 || gaps[1].Needed != 3 {
		t.Fatalf("Guesser gap = %+v, want assigned 1 need 3", gaps[1])
	}
	if ReadyToFire(gaps) {
		t.Fatal("expected not ready")
	}
}

func TestReadyToFire_WordHuntFull(t *testing.T) {
	specs := []seattemplate.PathSpec{
		{QueuePath: "ClueGiver", SizeForQueue: 2},
		{QueuePath: "Guesser", SizeForQueue: 4},
	}
	assignments := []Assignment{
		{UserID: "a", QueuePath: "ClueGiver"},
		{UserID: "b", QueuePath: "ClueGiver"},
		{UserID: "c", QueuePath: "Guesser"},
		{UserID: "d", QueuePath: "Guesser"},
		{UserID: "e", QueuePath: "Guesser"},
		{UserID: "f", QueuePath: "Guesser"},
	}
	gaps := ComputePathGaps(specs, assignments)
	if !ReadyToFire(gaps) {
		t.Fatalf("expected ready, gaps=%+v", gaps)
	}
	if TargetPlayerCount(specs) != 6 {
		t.Fatalf("target = %d, want 6", TargetPlayerCount(specs))
	}
}
