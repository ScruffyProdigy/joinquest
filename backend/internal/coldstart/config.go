package coldstart

import (
	"os"
	"strings"
	"time"
)

// EnabledEnv is the environment variable that turns cold-start seeding on.
const EnabledEnv = "COLD_START_SEEDING"

// RefitIntervalEnv overrides how often mode pairs are refitted.
const RefitIntervalEnv = "COLD_START_REFIT_INTERVAL"

// DefaultRefitInterval is how often the recompute runs when nothing overrides
// it. Six hours: the inputs are converged ratings, which move on the timescale
// of players accumulating matches, and a pair that crosses MinPairedPlayers
// four hours later than it might have costs nothing.
const DefaultRefitInterval = 6 * time.Hour

// EnabledFromEnv reports whether seeding should be served, defaulting to off.
//
// Off until the backtest says otherwise, which is the whole point of shipping
// it behind a flag: the numbers that decide whether seeding beats the flat
// prior — ResidualSD against FlatResidualSD, per pair — do not exist until
// real players have played two modes of one game, and no amount of reasoning
// here substitutes for reading them.
//
// Read per call rather than cached so the flag can be flipped by restarting a
// pod, and so a test can set it for the duration of one test. The cost is a
// map lookup on a path that is already making database queries.
func EnabledFromEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnabledEnv)), "true")
}

// RefitIntervalFromEnv returns the recompute interval.
//
// Note that this is not gated on EnabledFromEnv, and deliberately: fitting
// costs two queries per game against its own table every few hours, touches
// no rating anyone reads, and is the only way the flag above ever gets a
// basis to be flipped. A deployment that measured nothing until seeding was
// switched on would have to switch it on blind.
func RefitIntervalFromEnv() time.Duration {
	if v := strings.TrimSpace(os.Getenv(RefitIntervalEnv)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return DefaultRefitInterval
}
