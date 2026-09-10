// Command backtest scores rating models against held-out match history.
//
// It is a read-only tool. It walks a mode's rating_match_inputs log, trains an
// engine on a prefix, forecasts each held-out match from the ratings as they
// stood at that moment, and reports how well the forecast did — without ever
// touching player_ratings or nonplayer_ratings. Running it against production
// is safe by construction: it holds the store through
// ratingbacktest.Source, which has no write method (see internal/ratingbacktest).
//
// Its reason to exist is comparison. Every open question about the rating
// engine — whether a different model suits a mode better, how to calibrate a
// cold-start prior's sigma — is a matter of taste until two candidates can be
// run over the same history and scored. Pass more than one -engine and the
// report puts them side by side.
//
//	backtest -engine plackett-luce
//	backtest -game <uuid> -mode ranked -train-fraction 0.7
//	backtest -json | jq .
//
// Read the score together with the identifiability block underneath it. A
// modifier whose players-with-multiple-values is zero is confounded with
// player skill over this history, and no comparison run on it means what it
// appears to mean.
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
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/ratingbacktest"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func main() {
	var (
		databaseURL   = flag.String("database-url", "", "Database connection URL (defaults to $DATABASE_URL)")
		gameID        = flag.String("game", "", "Game id to score; defaults to every game with retained history")
		modeKey       = flag.String("mode", "", "Mode key to score; defaults to every mode of the selected game(s)")
		engineSpecs   = flag.String("engine", "plackett-luce", "Comma-separated Weng-Lin models to score and compare")
		trainFraction = flag.Float64("train-fraction", ratingbacktest.DefaultSplit.TrainFraction, "Share of each mode's history used for training, in (0,1)")
		trainMatches  = flag.Int("train-matches", 0, "Exact number of training matches; overrides -train-fraction when positive")
		asJSON        = flag.Bool("json", false, "Emit the report as JSON instead of a table")
		timeout       = flag.Duration("timeout", 10*time.Minute, "Overall timeout for the run")
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

	engines, err := buildEngines(*engineSpecs)
	if err != nil {
		log.Fatalf("Invalid -engine: %v", err)
	}

	split := ratingbacktest.Split{TrainFraction: *trainFraction, TrainMatches: *trainMatches}

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
		log.Fatal("No retained rating history matched the selection: nothing to score")
	}

	// RatingSource returns a rating.Store, which satisfies the read-only
	// ratingbacktest.Source. The write half comes along but is unreachable
	// through that interface — this is where "a dry run cannot mutate the
	// live cache" stops being a promise and becomes a type.
	harness, err := ratingbacktest.New(st.RatingSource(), split)
	if err != nil {
		log.Fatalf("Invalid split: %v", err)
	}

	report, err := harness.Run(ctx, targets, engines...)
	if err != nil {
		log.Fatalf("Backtest failed: %v", err)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			log.Fatalf("Failed to encode report: %v", err)
		}
		return
	}
	printReport(os.Stdout, report)
}

// buildEngines turns the -engine flag into the engines to compare, preserving
// the order given so the report's columns match what was asked for.
//
// Every name goes through rating.NewWengLin, which is the only family of
// models in the tree today; an unknown name is rejected there rather than
// quietly scoring nothing. A second family (JQ-231) adds a case here.
func buildEngines(spec string) ([]rating.Engine, error) {
	var engines []rating.Engine
	seen := make(map[string]bool)
	for _, name := range strings.Split(spec, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if seen[name] {
			// Scoring one engine twice would print two identical columns and
			// invite reading them as agreement between two models.
			return nil, fmt.Errorf("model %q listed more than once", name)
		}
		seen[name] = true

		engine, err := rating.NewWengLin(name)
		if err != nil {
			return nil, err
		}
		engines = append(engines, engine)
	}
	if len(engines) == 0 {
		return nil, fmt.Errorf("no models named")
	}
	return engines, nil
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

func printReport(w io.Writer, report ratingbacktest.Report) {
	if report.Split.TrainMatches > 0 {
		fmt.Fprintf(w, "split: first %d matches train, remainder held out\n\n", report.Split.TrainMatches)
	} else {
		fmt.Fprintf(w, "split: first %.0f%% of matches train, remainder held out\n\n", report.Split.TrainFraction*100)
	}

	for _, mode := range report.Modes {
		fmt.Fprintf(w, "%s / %s — %d matches (%d train, %d held out)\n",
			mode.Target.GameID, mode.Target.ModeKey, mode.Matches, mode.TrainMatches, mode.HeldOutMatches)

		if mode.HeldOutMatches == 0 {
			fmt.Fprintf(w, "  no held-out matches: nothing scored\n\n")
			continue
		}

		fmt.Fprintf(w, "  %-28s %8s %8s %8s %8s\n", "engine", "acc", "logloss", "brier", "scored")
		for _, es := range mode.Engines {
			s := es.Score
			logLoss, brier := "-", "-"
			if s.Probabilistic && s.ProbMatches > 0 {
				logLoss = fmt.Sprintf("%.4f", s.LogLoss)
				brier = fmt.Sprintf("%.4f", s.Brier)
			}
			fmt.Fprintf(w, "  %-28s %8.4f %8s %8s %8d\n", es.EngineID, s.Accuracy, logLoss, brier, s.HeldOut)
		}

		printModifiers(w, mode.Modifiers)
		fmt.Fprintln(w)
	}
}

// printModifiers prints the identifiability block beneath a mode's scores.
//
// It is not optional detail and it is not printed elsewhere: a modifier that
// never varies within a player is inseparable from that player's skill, so any
// score computed over that history is measuring a quantity nobody named. The
// reader has to meet both facts at once.
func printModifiers(w io.Writer, mods map[string]rating.ModifierReport) {
	if len(mods) == 0 {
		return
	}

	keys := make([]string, 0, len(mods))
	for k := range mods {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintf(w, "  modifiers:\n")
	for _, k := range keys {
		m := mods[k]
		note := ""
		if m.PlayersWithMultipleValues == 0 {
			note = "  ** confounded with player skill **"
		}
		fmt.Fprintf(w, "    %-26s matches=%-6d values=%-3d players-with-multiple-values=%-5d%s\n",
			k, m.Matches, m.DistinctValues, m.PlayersWithMultipleValues, note)
	}
}
