package rating

import "testing"

func solo(key string, r Rating, rank int) Side {
	return Side{Entrants: []Entrant{{Key: key, Rating: r}}, Rank: rank}
}

func TestRateOneVersusOneMovesWinnerUpAndLoserDown(t *testing.T) {
	e, err := NewWengLin("plackett-luce")
	if err != nil {
		t.Fatalf("NewWengLin: %v", err)
	}
	prior := e.Prior()

	got, err := e.Rate([]Side{
		solo("player:a", prior, 0), // winner
		solo("player:b", prior, 1),
	})
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}

	winner, loser := got[0][0], got[1][0]
	if winner.Mu <= prior.Mu {
		t.Errorf("winner mu = %v, want greater than prior %v", winner.Mu, prior.Mu)
	}
	if loser.Mu >= prior.Mu {
		t.Errorf("loser mu = %v, want less than prior %v", loser.Mu, prior.Mu)
	}
	// A match is evidence about both players, so uncertainty falls for each.
	if winner.Sigma >= prior.Sigma || loser.Sigma >= prior.Sigma {
		t.Errorf("sigma did not fall: winner %v, loser %v, prior %v",
			winner.Sigma, loser.Sigma, prior.Sigma)
	}
}

func TestRateTieLeavesEqualPlayersEqual(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	prior := e.Prior()

	got, err := e.Rate([]Side{
		solo("player:a", prior, 0),
		solo("player:b", prior, 0), // equal rank is a tie
	})
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if got[0][0].Mu != got[1][0].Mu {
		t.Errorf("tied equal players diverged: %v vs %v", got[0][0].Mu, got[1][0].Mu)
	}
	if got[0][0].Sigma >= prior.Sigma {
		t.Errorf("a tie is still evidence; sigma = %v, want below prior %v",
			got[0][0].Sigma, prior.Sigma)
	}
}

func TestRateFreeForAllOrdersByRank(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	prior := e.Prior()

	got, err := e.Rate([]Side{
		solo("player:a", prior, 0),
		solo("player:b", prior, 1),
		solo("player:c", prior, 2),
		solo("player:d", prior, 3),
	})
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1][0].Mu <= got[i][0].Mu {
			t.Errorf("placement %d mu %v not above placement %d mu %v",
				i-1, got[i-1][0].Mu, i, got[i][0].Mu)
		}
	}
}

func TestRateAsymmetricTeamsAreAccepted(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	p := e.Prior()

	got, err := e.Rate([]Side{
		{Entrants: []Entrant{{Key: "player:a", Rating: p}, {Key: "player:b", Rating: p}}, Rank: 0},
		{Entrants: []Entrant{{Key: "player:c", Rating: p}}, Rank: 1},
	})
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 1 {
		t.Fatalf("shape = %v, want [[2][1]]", got)
	}
}

// A non-player entrant is nothing special to the engine — this is what lets a
// co-op scenario and a side-advantage modifier ride the same equations.
func TestRateTreatsNonPlayerEntrantsIdentically(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	p := e.Prior()

	got, err := e.Rate([]Side{
		{Entrants: []Entrant{{Key: "player:a", Rating: p}, {Key: "player:b", Rating: p}}, Rank: 0},
		{Entrants: []Entrant{{Key: "scenario", Rating: p}}, Rank: 1},
	})
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if got[1][0].Mu >= p.Mu {
		t.Errorf("beaten scenario mu = %v, want below prior %v", got[1][0].Mu, p.Mu)
	}
}

func TestPriorMovesFirstMatchFarMoreThanHundredth(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	winner, loser := e.Prior(), e.Prior()

	var firstDelta, lastDelta float64
	for i := 0; i < 100; i++ {
		got, err := e.Rate([]Side{
			solo("player:a", winner, 0),
			solo("player:b", loser, 1),
		})
		if err != nil {
			t.Fatalf("Rate at %d: %v", i, err)
		}
		delta := got[0][0].Mu - winner.Mu
		if i == 0 {
			firstDelta = delta
		}
		lastDelta = delta
		winner, loser = got[0][0], got[1][0]
	}
	if firstDelta <= lastDelta*2 {
		t.Errorf("first match moved %v, hundredth moved %v; want the first far larger",
			firstDelta, lastDelta)
	}
}

func TestNewWengLinRejectsUnknownModel(t *testing.T) {
	if _, err := NewWengLin("elo"); err == nil {
		t.Fatal("NewWengLin(\"elo\") = nil error, want an error")
	}
}

func TestEngineIDNamesModel(t *testing.T) {
	e, _ := NewWengLin("plackett-luce")
	if e.ID() != "weng-lin/plackett-luce@1" {
		t.Errorf("ID() = %q, want weng-lin/plackett-luce@1", e.ID())
	}
}
