// Command ratingbeta fits each mode's performance variance from its own match
// history, and optionally records it.
//
// Beta is the constant that says how much a gap in skill actually predicts who
// wins. JQ-139 shipped one value for the whole catalog, which asserts that
// skill separates players exactly as well in a duel as in a high-luck party
// game. This measures it per mode instead, by running the backtest harness's
// prequential walk once per candidate beta and keeping the one that forecasts
// held-out matches best.
//
// What it is for is a question matchmaking cannot currently ask: how hard
// should we even try for this mode? A mode where skill barely moves the result
// should spend its budget on queue time and group fit, because strict skill
// matching there costs waiting and buys nothing a player would notice.
//
//	ratingbeta                                   # fit every mode, report only
//	ratingbeta -game <uuid> -mode ranked
//	ratingbeta -apply                            # fit and record what qualifies
//	ratingbeta -json | jq .
//
// Fitting is read-only; only -apply writes, and it writes nothing but
// mode_rating_constants. A recorded beta takes effect on the next replay sweep,
// which sees the mode's cached ratings carrying constants that no longer match
// and replays its whole history under the new value — ratings computed under
// different betas are not comparable, so nothing is carried across.
//
// A mode with too little history is reported and not recorded. That is the
// point of the floor: an argmin over a few dozen matches is noise with a
// decimal point, and the report says so rather than quietly fitting it.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/ratingbacktest"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// ratingModel is the model family every candidate is built in. Beta is a
// constant of a model, so fitting it only means anything against a fixed one;
// comparing models is what the backtest command is for.
const ratingModel = "plackett-luce"

func main() {
	var (
		databaseURL   = flag.String("database-url", "", "Database connection URL (defaults to $DATABASE_URL)")
		gameID        = flag.String("game", "", "Game id to fit; defaults to every game with retained history")
		modeKey       = flag.String("mode", "", "Mode key to fit; defaults to every mode of the selected game(s)")
		trainFraction = flag.Float64("train-fraction", ratingbacktest.DefaultSplit.TrainFraction, "Share of each mode's history used for training, in (0,1)")
		trainMatches  = flag.Int("train-matches", 0, "Exact number of training matches; overrides -train-fraction when positive")
		minMatches    = flag.Int("min-matches", ratingbacktest.MinProbMatches, "Scored held-out matches required before a fit is trustworthy enough to record")
		multipliers   = flag.String("multipliers", "", "Comma-separated candidate betas, as multiples of the default; defaults to the standard grid")
		apply         = flag.Bool("apply", false, "Record every qualifying estimate, replacing any earlier measurement")
		asJSON        = flag.Bool("json", false, "Emit the report as JSON instead of a table")
		timeout       = flag.Duration("timeout", 30*time.Minute, "Overall timeout for the run")
	)
	flag.Parse()

	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
		if *databaseURL == "" {
			log.Fatal("Database URL is required. Set DATABASE_URL or use -database-url")
		}
	}
	if *modeKey != "" && *gameID == "" {
		log.Fatal("-mode selects a mode within a game: pass -game as well")
	}

	mults, err := parseMultipliers(*multipliers)
	if err != nil {
		log.Fatalf("Invalid -multipliers: %v", err)
	}

	db, err := sql.Open("postgres", *databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	st := store.New(db)

	targets, err := resolveTargets(ctx, st, *gameID, *modeKey)
	if err != nil {
		log.Fatalf("Failed to resolve targets: %v", err)
	}
	if len(targets) == 0 {
		log.Fatal("No retained rating history matched the selection: nothing to fit")
	}

	harness, err := ratingbacktest.New(st.RatingSource(), ratingbacktest.Split{
		TrainFraction: *trainFraction,
		TrainMatches:  *trainMatches,
	})
	if err != nil {
		log.Fatalf("Invalid split: %v", err)
	}

	grid := ratingbacktest.BetaGrid{
		Default:        rating.DefaultBeta,
		Multipliers:    mults,
		MinProbMatches: *minMatches,
	}
	newEngine := func(beta float64) (rating.Engine, error) {
		return rating.NewWengLinBeta(ratingModel, beta)
	}

	estimates := make([]ratingbacktest.BetaEstimate, 0, len(targets))
	for _, target := range targets {
		est, err := harness.EstimateBeta(ctx, target, newEngine, grid)
		if err != nil {
			log.Fatalf("Failed to fit %s/%s: %v", target.GameID, target.ModeKey, err)
		}
		estimates = append(estimates, est)
	}

	var applied []string
	if *apply {
		applied, err = record(ctx, st, estimates, newEngine)
		if err != nil {
			log.Fatalf("Failed to record estimates: %v", err)
		}
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Estimates []ratingbacktest.BetaEstimate `json:"estimates"`
			Applied   []string                      `json:"applied,omitempty"`
		}{estimates, applied}); err != nil {
			log.Fatalf("Failed to encode report: %v", err)
		}
		return
	}
	printEstimates(os.Stdout, estimates, rating.DefaultBeta)
	printApplied(os.Stdout, *apply, applied, estimates)
}

// parseMultipliers turns the -multipliers flag into a grid. An empty flag
// leaves the standard grid in place rather than producing an empty one.
func parseMultipliers(spec string) ([]float64, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	var out []float64
	for _, field := range strings.Split(spec, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		m, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", field)
		}
		if !(m > 0) {
			return nil, fmt.Errorf("multiplier %v is not positive", m)
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no multipliers given")
	}
	return out, nil
}

// resolveTargets expands the -game/-mode selection into concrete targets,
// enumerating from the input log when the selection is open.
func resolveTargets(ctx context.Context, st *store.Store, gameID, modeKey string) ([]ratingbacktest.Target, error) {
	modes, err := st.ListRatedModes(ctx)
	if err != nil {
		return nil, err
	}

	targets := make([]ratingbacktest.Target, 0, len(modes))
	for _, m := range modes {
		id := m.GameID.String()
		if gameID != "" && id != gameID {
			continue
		}
		if modeKey != "" && m.ModeKey != modeKey {
			continue
		}
		targets = append(targets, ratingbacktest.Target{GameID: id, ModeKey: m.ModeKey})
	}
	return targets, nil
}

// record writes every measured estimate and returns the modes it touched.
//
// Unmeasured estimates are skipped rather than written at the default: a row
// here claims a measurement, and a mode that has not earned one must stay
// absent so it keeps reading as unmeasured (see store.ModeRatingConstants).
func record(ctx context.Context, st *store.Store, estimates []ratingbacktest.BetaEstimate, newEngine ratingbacktest.BetaEngine) ([]string, error) {
	var applied []string
	for _, est := range estimates {
		if !est.Measured {
			continue
		}
		gameID, err := parseGameID(est.Target.GameID)
		if err != nil {
			return applied, err
		}
		engine, err := newEngine(est.Beta)
		if err != nil {
			return applied, err
		}
		if err := st.SetModeRatingConstants(ctx, gameID, est.Target.ModeKey, store.ModeRatingConstants{
			Beta:        est.Beta,
			EngineID:    engine.ID(),
			SampleSize:  est.ProbMatches,
			EstimatedAt: time.Now().UTC(),
		}); err != nil {
			return applied, fmt.Errorf("record %s/%s: %w", est.Target.GameID, est.Target.ModeKey, err)
		}
		applied = append(applied, est.Target.GameID+"/"+est.Target.ModeKey)
	}
	return applied, nil
}

func printEstimates(w io.Writer, estimates []ratingbacktest.BetaEstimate, defaultBeta float64) {
	fmt.Fprintf(w, "default beta: %.4f\n\n", defaultBeta)

	for _, est := range estimates {
		fmt.Fprintf(w, "%s / %s — %d matches (%d held out, %d scored)\n",
			est.Target.GameID, est.Target.ModeKey, est.Matches, est.HeldOut, est.ProbMatches)

		if !est.Measured {
			// The sample size is on the line above whether or not anything was
			// measured, so "we did not fit this" never reads as "we have no
			// idea how close it came".
			fmt.Fprintf(w, "  unmeasured: %s\n", est.Unmeasured)
			fmt.Fprintf(w, "  keeping the default beta %.4f\n", est.Default)
		} else {
			edge := ""
			if est.AtGridEdge {
				edge = "  ** at the edge of the grid: the true value may lie beyond it **"
			}
			fmt.Fprintf(w, "  beta %.4f (%.3fx default), fitted on %d match(es)%s\n",
				est.Beta, est.Beta/est.Default, est.ProbMatches, edge)
			fmt.Fprintf(w, "  log loss %.4f vs %.4f at the default\n", est.LogLoss, est.DefaultLogLoss)
		}

		printCurve(w, est)
		fmt.Fprintln(w)
	}
}

// printCurve prints every candidate, not just the winner.
//
// The shape carries information the minimum does not: a curve that barely
// moves across a 64-fold change in beta means this history cannot tell a
// skill-driven mode from a luck-driven one at all, and its argmin is noise
// wearing a decimal point. A reader has to be able to see that.
func printCurve(w io.Writer, est ratingbacktest.BetaEstimate) {
	if len(est.Candidates) == 0 {
		return
	}
	fmt.Fprintf(w, "  %10s %10s %8s %8s\n", "beta", "logloss", "brier", "acc")
	for _, c := range est.Candidates {
		marker := "  "
		switch {
		case est.Measured && c.Beta == est.Beta:
			marker = "->"
		case c.Beta == est.Default:
			marker = " ."
		}
		fmt.Fprintf(w, "%s%10.4f %10.4f %8.4f %8.4f\n",
			marker, c.Beta, c.Score.LogLoss, c.Score.Brier, c.Score.Accuracy)
	}
}

func printApplied(w io.Writer, apply bool, applied []string, estimates []ratingbacktest.BetaEstimate) {
	if !apply {
		var measured int
		for _, est := range estimates {
			if est.Measured {
				measured++
			}
		}
		fmt.Fprintf(w, "%d of %d mode(s) qualify. Nothing was written; pass -apply to record them.\n", measured, len(estimates))
		return
	}
	if len(applied) == 0 {
		fmt.Fprintln(w, "No mode qualified, so nothing was recorded.")
		return
	}
	fmt.Fprintf(w, "Recorded %d mode(s):\n", len(applied))
	for _, name := range applied {
		fmt.Fprintf(w, "  %s\n", name)
	}
	// Saying this out loud because the write looks like it did nothing: the
	// ratings a player sees do not move until the mode is replayed, and the
	// replay is somebody else's tick.
	fmt.Fprintln(w, "Each mode's ratings are now stale and will be replayed in full by the next rating sweep.")
}

// parseGameID keeps the uuid dependency in one place: Target carries the game
// id as the string the harness works in, and the store wants a uuid.UUID.
func parseGameID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse game id %q: %w", id, err)
	}
	return parsed, nil
}
