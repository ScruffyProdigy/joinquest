package rating_test

import (
	"reflect"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/rating/openskill/rating"
	"github.com/scruffyprodigy/joinquest/internal/rating/openskill/types"
)

// TestRateIsDeterministic is the property JQ-139 depends on: replay
// recomputes every rating from the match log, so the same input must always
// produce byte-identical output. Map iteration in an ordering-sensitive path
// or a stray RNG would break it, and both fail here rather than silently in
// production.
func TestRateIsDeterministic(t *testing.T) {
	teams := []types.Team{
		{rating.New(), rating.New(), rating.New()},
		{rating.New(), rating.New()},
		{rating.New()},
	}

	first := rating.Rate(teams, &types.OpenSkillOptions{})
	for i := 0; i < 200; i++ {
		got := rating.Rate(teams, &types.OpenSkillOptions{})
		if !reflect.DeepEqual(first, got) {
			t.Fatalf("run %d differs from first run:\nfirst = %+v\ngot   = %+v", i, first, got)
		}
	}
}
