package graph

import (
	"time"

	"github.com/scruffyprodigy/joinquest/internal/gameclient"
)

func (r *Resolver) eligibilityCache() *gameclient.EligibilityCache {
	if r.EligibilityCache != nil {
		return r.EligibilityCache
	}
	return gameclient.NewEligibilityCache(gameclient.NewClient(), 5*time.Second)
}
