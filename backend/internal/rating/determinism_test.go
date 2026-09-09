package rating

import "testing"

// TestRateIsDeterministic is the property replay depends on: replay
// recomputes every rating from the match log, so the same input must always
// produce byte-identical output. Map iteration in an ordering-sensitive path
// or a stray RNG would break it, and both fail here rather than silently in
// production. It runs through Engine rather than the go-openskill package
// directly, since Engine is the surface the rest of this package actually
// depends on.
func TestRateIsDeterministic(t *testing.T) {
	e, err := NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	prior := e.Prior()

	sides := []Side{
		{Entrants: []Entrant{
			{Key: "player:a", Rating: prior},
			{Key: "player:b", Rating: prior},
			{Key: "player:c", Rating: prior},
		}, Rank: 0},
		{Entrants: []Entrant{
			{Key: "player:d", Rating: prior},
			{Key: "player:e", Rating: prior},
		}, Rank: 1},
		{Entrants: []Entrant{
			{Key: "player:f", Rating: prior},
		}, Rank: 2},
	}

	first, err := e.Rate(sides)
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	for i := 0; i < 200; i++ {
		got, err := e.Rate(sides)
		if err != nil {
			t.Fatalf("Rate at %d: %v", i, err)
		}
		if !ratingsEqual(first, got) {
			t.Fatalf("run %d differs from first run:\nfirst = %+v\ngot   = %+v", i, first, got)
		}
	}
}

func ratingsEqual(a, b [][]Rating) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
