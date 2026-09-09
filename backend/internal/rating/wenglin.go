package rating

import (
	"fmt"

	openskillrating "github.com/intinig/go-openskill/rating"
	"github.com/intinig/go-openskill/types"
)

// wengLin implements Engine using the go-openskill Weng-Lin (OpenSkill) package.
type wengLin struct {
	model string
}

// NewWengLin returns an Engine backed by the go-openskill implementation.
//
// model must be "plackett-luce" — currently the only model the package
// implements. Names like "bradley-terry" and "thurstone-mosteller" are not
// a recognized-but-unavailable case; they are rejected with the same plain
// unknown-model error as any made-up string, e.g. "elo". The string
// parameter stays even though only one value is accepted: NewWengLin has no
// caller outside this codebase — no public API, no game-developer surface,
// nothing external that reads a model name — so configurability across
// formulas would only be us talking to ourselves. If a second model is ever
// pulled in, swapping it in means writing a new Engine implementation
// behind this same interface, which the interface already supports; a name
// is added the day the formula behind it exists, not before.
func NewWengLin(model string) (Engine, error) {
	switch model {
	case "plackett-luce":
		return &wengLin{model: model}, nil
	default:
		return nil, fmt.Errorf("rating: unknown model %q", model)
	}
}

// Prior returns the package's default starting rating.
func (w *wengLin) Prior() Rating {
	r := openskillrating.New()
	return Rating{Mu: r.Mu, Sigma: r.Sigma}
}

// Rate converts sides into the package's teams-plus-ranks shape, rates
// them, and maps the result back preserving the input's shape and order.
func (w *wengLin) Rate(sides []Side) ([][]Rating, error) {
	teams := make([]types.Team, len(sides))
	ranks := make([]int, len(sides))
	for i, side := range sides {
		team := make(types.Team, len(side.Entrants))
		for j, entrant := range side.Entrants {
			team[j] = types.Rating{Mu: entrant.Rating.Mu, Sigma: entrant.Rating.Sigma}
		}
		teams[i] = team
		ranks[i] = side.Rank
	}

	rated := openskillrating.Rate(teams, &types.OpenSkillOptions{Rank: ranks})

	out := make([][]Rating, len(rated))
	for i, team := range rated {
		row := make([]Rating, len(team))
		for j, r := range team {
			row[j] = Rating{Mu: r.Mu, Sigma: r.Sigma}
		}
		out[i] = row
	}
	return out, nil
}

// ID names the model and its constants version, for example
// "weng-lin/plackett-luce@1". The "@1" suffix is a hand-bumped constants
// version, not derived from the go-openskill package, so replayed history
// records exactly which constants produced a given rating.
func (w *wengLin) ID() string {
	return "weng-lin/" + w.model + "@1"
}
