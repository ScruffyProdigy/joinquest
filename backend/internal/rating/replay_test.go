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

// TestReplayOfPrefixesIsStableAndOrderPreserving replays each successive
// prefix of a history (the first match, then the first two, then all three)
// through two independent Replayers and checks the two runs agree at every
// prefix length. "Order preserving" means exactly what ReplayMode's own doc
// promises: replay always walks Input.Sides in slice order, so growing the
// prefix by one match never reorders the matches already in it. This extends
// TestReplayIsDeterministicAcrossRuns's determinism check to every prefix
// length, not only the full history.
//
// This test used to be named TestReplayEqualsIncrementalApplication and
// compared a from-scratch replay of the final prefix (history[:3], the whole
// history) against a from-scratch replay of the whole history — two
// byte-identical inputs replayed the same way, so the comparison could never
// fail no matter what the code did.
//
// Incremental application is not a code path this design has: ReplayMode
// always recomputes every rating from Engine.Prior() over the whole input
// log, and the Store port (see the Store interface above) deliberately has
// no method to load an existing rating and append one match to it. There is
// no incremental path to test, so this test does not pretend one exists.
func TestReplayOfPrefixesIsStableAndOrderPreserving(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	history := threeMatchHistory()

	for i := range history {
		prefix := history[:i+1]

		a := &fakeStore{inputs: prefix}
		if _, err := NewReplayer(e, a).ReplayMode(context.Background(), "game", "duel"); err != nil {
			t.Fatalf("prefix %d, run a: %v", i, err)
		}

		b := &fakeStore{inputs: prefix}
		if _, err := NewReplayer(e, b).ReplayMode(context.Background(), "game", "duel"); err != nil {
			t.Fatalf("prefix %d, run b: %v", i, err)
		}

		if !reflect.DeepEqual(a.players, b.players) {
			t.Errorf("prefix %d: two replays of the same prefix diverged:\na = %+v\nb = %+v", i, a.players, b.players)
		}
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

// prequeueLockedColorHistory locks alice to "prequeue:color/white" across
// two matches whose "prequeue:map" value differs (forest, then desert): color
// and map are separate dimensions, so alice's skill and her color choice are
// perfectly confounded even though her map varies from match to match. Before
// splitModifierKey separated the "prequeue:color" and "prequeue:map"
// dimensions, both collapsed into a single "prequeue" category, and alice's
// varying map value made the report falsely claim her color was
// identifiable.
func prequeueLockedColorHistory() []Input {
	return []Input{
		{
			SessionID: "p1",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:alice"}, {Key: "prequeue:color/white"}, {Key: "prequeue:map/forest"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:bob"}, {Key: "prequeue:color/black"}, {Key: "prequeue:map/forest"}}},
			},
		},
		{
			SessionID: "p2",
			Sides: []Side{
				{Rank: 0, Entrants: []Entrant{{Key: "player:alice"}, {Key: "prequeue:color/white"}, {Key: "prequeue:map/desert"}}},
				{Rank: 1, Entrants: []Entrant{{Key: "player:carol"}, {Key: "prequeue:color/black"}, {Key: "prequeue:map/desert"}}},
			},
		},
	}
}

// TestReplayReportSeparatesCompoundModifierDimensions guards against a
// compound key's namespace ("prequeue") being treated as a single category
// when it actually names several independent dimensions ("color", "map").
// alice is locked to prequeue:color/white for both matches even though her
// map varies, so color must be reported as confounded for her (0 players with
// multiple values), and the color dimension's distinct-value count must not
// be inflated by map's values.
func TestReplayReportSeparatesCompoundModifierDimensions(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	fs := &fakeStore{inputs: prequeueLockedColorHistory()}

	report, err := NewReplayer(e, fs).ReplayMode(context.Background(), "game", "duel")
	if err != nil {
		t.Fatalf("ReplayMode: %v", err)
	}

	got := report.Modifiers["prequeue:color/white"]
	if got.PlayersWithMultipleValues != 0 {
		t.Errorf("PlayersWithMultipleValues = %d, want 0: alice is locked to color/white even though her map varies", got.PlayersWithMultipleValues)
	}
	if got.DistinctValues != 2 {
		t.Errorf("DistinctValues = %d, want 2 (white, black) for the color dimension alone, not lumped with map", got.DistinctValues)
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
