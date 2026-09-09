package rating

import (
	"context"
	"reflect"
	"testing"
)

// fakeStore is an in-memory Store, so determinism is tested without Postgres.
type fakeStore struct {
	inputs    []Input
	players   map[string]Rating
	entities  map[string]Rating
	saveCalls int
}

func (f *fakeStore) ListInputs(context.Context, string, string) ([]Input, error) {
	return f.inputs, nil
}
func (f *fakeStore) SaveAll(_ context.Context, _ string, _ string, _ string, players, entities map[string]Rating) error {
	f.players, f.entities = players, entities
	f.saveCalls++
	return nil
}

// threeMatchHistory is a plain duel history: three players, three matches,
// every player has played at least once by the end.
func threeMatchHistory() []Input {
	return []Input{
		{
			SessionID: "s1",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:alice"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:bob"}}},
			},
		},
		{
			SessionID: "s2",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:bob"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:carol"}}},
			},
		},
		{
			SessionID: "s3",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:alice"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:carol"}}},
			},
		},
	}
}

// mixedSideHistory is a chess history where every player appears on both
// "seat:White" and "seat:Black" across the two matches: the seat modifier is
// identifiable because it varies within each player.
func mixedSideHistory() []Input {
	return []Input{
		{
			SessionID: "c1",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:alice"}, {Key: "seat:White"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:bob"}, {Key: "seat:Black"}}},
			},
		},
		{
			SessionID: "c2",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:bob"}, {Key: "seat:White"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:alice"}, {Key: "seat:Black"}}},
			},
		},
	}
}

// lockedSideHistory is a chess history where every player is locked to one
// seat for the whole replay: skill and seat are perfectly confounded, and
// the report must say so rather than producing confident nonsense.
func lockedSideHistory() []Input {
	return []Input{
		{
			SessionID: "l1",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:alice"}, {Key: "seat:White"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:bob"}, {Key: "seat:Black"}}},
			},
		},
		{
			SessionID: "l2",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:alice"}, {Key: "seat:White"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:carol"}, {Key: "seat:Black"}}},
			},
		},
	}
}

func TestReplayIsDeterministicAcrossRuns(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")

	var first map[string]Rating
	for i := 0; i < 25; i++ {
		fs := &fakeStore{inputs: threeMatchHistory()}
		r := NewReplayer(e, fs)
		if _, err := r.ReplayMode(context.Background(), "game", "duel"); err != nil {
			t.Fatalf("ReplayMode: %v", err)
		}
		if i == 0 {
			first = fs.players
			continue
		}
		if !reflect.DeepEqual(first, fs.players) {
			t.Fatalf("run %d differs:\nfirst = %+v\ngot   = %+v", i, first, fs.players)
		}
	}
}

// The cache must equal the replay: applying matches one at a time as they
// arrive has to land on the same numbers as recomputing the whole history.
func TestReplayEqualsIncrementalApplication(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	history := threeMatchHistory()

	full := &fakeStore{inputs: history}
	if _, err := NewReplayer(e, full).ReplayMode(context.Background(), "game", "duel"); err != nil {
		t.Fatalf("full replay: %v", err)
	}

	incremental := &fakeStore{}
	for i := range history {
		incremental.inputs = history[:i+1]
		if _, err := NewReplayer(e, incremental).ReplayMode(context.Background(), "game", "duel"); err != nil {
			t.Fatalf("incremental replay at %d: %v", i, err)
		}
	}

	if !reflect.DeepEqual(full.players, incremental.players) {
		t.Errorf("full = %+v, incremental = %+v", full.players, incremental.players)
	}
}

func TestReplayStartsUnratedEntrantsFromThePrior(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	fs := &fakeStore{inputs: threeMatchHistory()}

	if _, err := NewReplayer(e, fs).ReplayMode(context.Background(), "game", "duel"); err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}
	for key, got := range fs.players {
		if got.Sigma >= e.Prior().Sigma {
			t.Errorf("%s sigma = %v, want below the prior %v after playing",
				key, got.Sigma, e.Prior().Sigma)
		}
	}
}

func TestReplayReportsWithinPlayerVariationPerModifier(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")

	// Every player appears on both sides: the modifier is identifiable.
	mixed := &fakeStore{inputs: mixedSideHistory()}
	report, err := NewReplayer(e, mixed).ReplayMode(context.Background(), "game", "chess")
	if err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}
	if got := report.Modifiers["seat:White"].PlayersWithMultipleValues; got == 0 {
		t.Error("mixed history should report players who appeared on both sides")
	}

	// Every player is locked to one side: skill and side are confounded, and
	// the report must say so rather than producing confident nonsense.
	locked := &fakeStore{inputs: lockedSideHistory()}
	report, err = NewReplayer(e, locked).ReplayMode(context.Background(), "game", "chess")
	if err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}
	if got := report.Modifiers["seat:White"].PlayersWithMultipleValues; got != 0 {
		t.Errorf("locked history reported %d mixed players, want 0", got)
	}
}
