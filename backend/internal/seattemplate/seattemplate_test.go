package seattemplate

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustExpand(t *testing.T, template string) []Leaf {
	t.Helper()
	leaves, err := Expand(json.RawMessage(template))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return leaves
}

func seatKeys(leaves []Leaf) []string {
	out := make([]string, len(leaves))
	for i, leaf := range leaves {
		out[i] = leaf.SeatKey
	}
	return out
}

func TestExpandDuelCountTwo(t *testing.T) {
	leaves := mustExpand(t, `{"count":2}`)
	if got := seatKeys(leaves); got[0] != "1" || got[1] != "2" {
		t.Fatalf("seat keys = %v, want [1 2]", got)
	}
	if leaves[0].QueuePath != "" || leaves[1].QueuePath != "" {
		t.Fatalf("expected default queue path, got %+v", leaves)
	}
}

func TestExpandThreeByThree(t *testing.T) {
	leaves := mustExpand(t, `{"Team":{"count":2,"Seat":{"count":3}}}`)
	want := []string{
		"Team-1-Seat-1", "Team-1-Seat-2", "Team-1-Seat-3",
		"Team-2-Seat-1", "Team-2-Seat-2", "Team-2-Seat-3",
	}
	got := seatKeys(leaves)
	if len(got) != len(want) {
		t.Fatalf("got %d seats, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("seat[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
	for _, leaf := range leaves {
		if strings.HasPrefix(leaf.SeatKey, "Team-1-") && leaf.AffinityKey != "Team:1" {
			t.Fatalf("seat %q affinity = %q, want Team:1", leaf.SeatKey, leaf.AffinityKey)
		}
		if strings.HasPrefix(leaf.SeatKey, "Team-2-") && leaf.AffinityKey != "Team:2" {
			t.Fatalf("seat %q affinity = %q, want Team:2", leaf.SeatKey, leaf.AffinityKey)
		}
	}
}

func TestExpandEmptyObjectVsCountOne(t *testing.T) {
	leaves := mustExpand(t, `{
		"Team":{"count":2,
			"DPS":{"count":2},
			"Tank":{"count":2},
			"Support":{}
		}
	}`)
	if len(leaves) != 10 {
		t.Fatalf("expected 10 seats, got %d", len(leaves))
	}
	paths := map[string]int{}
	for _, leaf := range leaves {
		paths[leaf.QueuePath]++
	}
	if paths["DPS"] != 4 || paths["Tank"] != 4 || paths["Support"] != 2 {
		t.Fatalf("unexpected queue paths: %+v", paths)
	}
}

func TestExpandRejectsFlatSeats(t *testing.T) {
	_, err := Expand(json.RawMessage(`{"seats":[{"key":"a"}]}`))
	if err == nil {
		t.Fatal("expected error for flat seats[]")
	}
}

func TestExpandNameArray(t *testing.T) {
	leaves := mustExpand(t, `{"ClueGiver":{"name":["Red","Blue","Green"]}}`)
	want := []string{"ClueGiver-Red", "ClueGiver-Blue", "ClueGiver-Green"}
	got := seatKeys(leaves)
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("seat[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpandWordHuntPartyTemplate(t *testing.T) {
	leaves := mustExpand(t, `{
		"ClueGiver":{"displayName":"Clue Giver","name":["Red","Blue","Green"],"min":2,"max":3,"sizeForQueue":2},
		"Guesser":{"count":6,"min":2,"max":6,"sizeForQueue":4}
	}`)
	want := []string{
		"ClueGiver-Red", "ClueGiver-Blue", "ClueGiver-Green",
		"Guesser-1", "Guesser-2", "Guesser-3", "Guesser-4", "Guesser-5", "Guesser-6",
	}
	got := seatKeys(leaves)
	if len(got) != len(want) {
		t.Fatalf("got %d seats %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("seat[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	paths := map[string]int{}
	for _, leaf := range leaves {
		paths[leaf.QueuePath]++
	}
	if paths["ClueGiver"] != 3 || paths["Guesser"] != 6 {
		t.Fatalf("unexpected queue paths: %+v", paths)
	}
}

func TestExpandCodenamesAffinity(t *testing.T) {
	leaves := mustExpand(t, `{"Team":{"count":2,"SpyMaster":{},"Guesser":{"count":3}}}`)
	if len(leaves) != 8 {
		t.Fatalf("got %d seats", len(leaves))
	}
	for _, leaf := range leaves {
		switch {
		case strings.HasPrefix(leaf.SeatKey, "Team-1-"):
			if leaf.AffinityKey != "Team:1" {
				t.Fatalf("seat %q affinity = %q, want Team:1", leaf.SeatKey, leaf.AffinityKey)
			}
		case strings.HasPrefix(leaf.SeatKey, "Team-2-"):
			if leaf.AffinityKey != "Team:2" {
				t.Fatalf("seat %q affinity = %q, want Team:2", leaf.SeatKey, leaf.AffinityKey)
			}
		default:
			t.Fatalf("unexpected seat %q", leaf.SeatKey)
		}
	}
}

func TestExpandSiegeOffenseDefense(t *testing.T) {
	leaves := mustExpand(t, `{
		"Offense":{"Wizard":{},"Warrior":{"count":2}},
		"Defense":{"Warrior":{"count":2}}
	}`)
	want := []string{
		"Defense-Warrior-1", "Defense-Warrior-2",
		"Offense-Warrior-1", "Offense-Warrior-2", "Offense-Wizard",
	}
	got := seatKeys(leaves)
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("seat[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpandNamePathIdentifiesEquivalenceClasses(t *testing.T) {
	tests := []struct {
		name     string
		template string
		// seatKey -> expected NamePath
		want map[string]string
	}{
		{
			name:     "flat template is one class",
			template: `{"count": 3}`,
			want:     map[string]string{"1": "", "2": "", "3": ""},
		},
		{
			name:     "symmetric teams are one class",
			template: `{"Team":{"count":2,"Seat":{"count":2}}}`,
			want: map[string]string{
				"Team-1-Seat-1": "Team/Seat",
				"Team-2-Seat-2": "Team/Seat",
			},
		},
		{
			name:     "distinct roles are distinct classes",
			template: `{"Team":{"count":2,"SpyMaster":{},"Guesser":{"count":2}}}`,
			want: map[string]string{
				"Team-1-SpyMaster": "Team/SpyMaster",
				"Team-2-Guesser-1": "Team/Guesser",
			},
		},
		{
			name:     "asymmetric sides are distinct classes",
			template: `{"White":{},"Black":{}}`,
			want:     map[string]string{"White": "White", "Black": "Black"},
		},
		{
			// A kind whose name ends in a digit must survive intact. This is the
			// case that rules out deriving the class by trimming the seat key.
			name:     "kind ending in a digit is not truncated",
			template: `{"Player2":{"count":2}}`,
			want:     map[string]string{"Player2-1": "Player2", "Player2-2": "Player2"},
		},
		{
			// Explicit names are instance labels, like indices: same class.
			name:     "named seats share their kind's class",
			template: `{"Seat":{"name":["Red","Blue"]}}`,
			want:     map[string]string{"Seat-Red": "Seat", "Seat-Blue": "Seat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leaves := mustExpand(t, tt.template)
			got := make(map[string]string, len(leaves))
			for _, leaf := range leaves {
				got[leaf.SeatKey] = leaf.NamePath
			}
			for seatKey, want := range tt.want {
				if got[seatKey] != want {
					t.Errorf("seat %q NamePath = %q, want %q", seatKey, got[seatKey], want)
				}
			}
		})
	}
}
