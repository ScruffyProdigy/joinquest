// Package rating estimates player skill from match outcomes.
//
// The engine is deliberately ignorant. It sees sides holding keyed entrants
// with ratings, and a rank per side; it does not know which entrants are
// people. That is what lets cooperative scenarios and side-advantage modifiers
// ride the same equations as ordinary players, and what lets the model be
// swapped without touching any caller.
package rating

// Rating is a skill estimate and its uncertainty.
type Rating struct {
	Mu    float64
	Sigma float64
}

// Entrant is one rated participant in a side. Key is opaque here: the store
// resolves "player:<uuid>" against player_ratings and everything else —
// "scenario", "seat:Team/SpyMaster", "prequeue:color/white" — against
// nonplayer_ratings.
type Entrant struct {
	Key    string
	Rating Rating
}

// Side is one team in a match. Rank 0 is best; equal ranks are a tie.
type Side struct {
	Entrants []Entrant
	Rank     int
}

// Engine turns a match outcome into updated ratings.
type Engine interface {
	// Prior is the starting rating for an entrant with no history: high
	// uncertainty, so a first match moves it substantially and a hundredth
	// barely does.
	Prior() Rating

	// Rate returns updated ratings in exactly the shape of the input — one
	// slice per side, one rating per entrant, in order.
	Rate(sides []Side) ([][]Rating, error)

	// ID names the model and its constants, for example
	// "weng-lin/plackett-luce@1". It is recorded with every update so a change
	// of engine or constants is detectable rather than silent.
	ID() string
}
