// Command coldstartfit measures how well one mode of a game predicts another,
// and prints the result.
//
// This is the report the COLD_START_SEEDING flag is supposed to be decided
// from. Seeding ships off (see internal/coldstart), and the condition for
// turning it on is that the seeding beats the flat prior on real players — a
// claim nobody can check without reading residual against flat-residual, per
// pair, on real data. Without this command the numbers would exist only in a
// table nobody looks at, and the flag could only ever be flipped on a hunch.
//
// It does not write by default. A refit over a game's ratings is exactly what
// the scheduled recompute does, and being able to see the numbers first —
// against production, without changing what production serves — is the whole
// point. Pass -write to persist, which is only useful for forcing a refit
// ahead of the next tick.
//
//	coldstartfit                       # every game, printed, nothing written
//	coldstartfit -game <uuid>
//	coldstartfit -json | jq .
//	coldstartfit -write                # persist, as the scheduled refit would
//
// Read residual against flat: a pair whose residual is not below the flat
// prior's has been measured and does not work, and the "usable" column says
// so. A pair below coldstart.MinPairedPlayers reads as unusable however good
// its correlation looks, which at that size is mostly luck.
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
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/scruffyprodigy/joinquest/internal/coldstart"
	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// gameFit is one game's measured pairs, as printed and as encoded to JSON.
type gameFit struct {
	GameID uuid.UUID             `json:"gameId"`
	Pairs  []coldstart.PairStats `json:"pairs"`
}

func main() {
	var (
		databaseURL = flag.String("database-url", "", "Database connection URL (defaults to $DATABASE_URL)")
		gameID      = flag.String("game", "", "Game id to fit; defaults to every game with cached ratings")
		write       = flag.Bool("write", false, "Persist the fit, as the scheduled recompute would; off by default")
		asJSON      = flag.Bool("json", false, "Emit the report as JSON instead of a table")
		timeout     = flag.Duration("timeout", 10*time.Minute, "Overall timeout for the run")
	)
	flag.Parse()

	if *databaseURL == "" {
		*databaseURL = os.Getenv("DATABASE_URL")
		if *databaseURL == "" {
			log.Fatal("Database URL is required. Set DATABASE_URL or use -database-url")
		}
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

	games, err := resolveGames(ctx, st, *gameID)
	if err != nil {
		log.Fatalf("Failed to resolve games: %v", err)
	}
	if len(games) == 0 {
		log.Fatal("No cached player ratings matched the selection: nothing to fit")
	}

	fits := make([]gameFit, 0, len(games))
	for _, id := range games {
		ratings, err := st.ListConvergedRatings(ctx, id, coldstart.ConvergedSigma)
		if err != nil {
			log.Fatalf("Failed to read ratings for %s: %v", id, err)
		}
		pairs := coldstart.FitPairs(ratings)
		fits = append(fits, gameFit{GameID: id, Pairs: pairs})

		if *write {
			if err := st.SaveModePairStats(ctx, id, time.Now().UTC(), pairs); err != nil {
				log.Fatalf("Failed to save pairs for %s: %v", id, err)
			}
		}
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(fits); err != nil {
			log.Fatalf("Failed to encode report: %v", err)
		}
		return
	}
	printFits(os.Stdout, fits, *write)
}

// resolveGames turns the -game flag into the games to fit.
func resolveGames(ctx context.Context, st *store.Store, gameID string) ([]uuid.UUID, error) {
	if gameID == "" {
		return st.ListGamesWithRatings(ctx)
	}
	id, err := uuid.Parse(gameID)
	if err != nil {
		return nil, fmt.Errorf("parse -game %q: %w", gameID, err)
	}
	return []uuid.UUID{id}, nil
}

func printFits(w io.Writer, fits []gameFit, wrote bool) {
	if wrote {
		fmt.Fprintf(w, "wrote the fit below; the next scheduled refit will replace it\n\n")
	} else {
		fmt.Fprintf(w, "dry run: nothing written (pass -write to persist)\n\n")
	}
	fmt.Fprintf(w, "seeding is served only when %s=true; usable=no means measured and not good enough\n\n", coldstart.EnabledEnv)

	for _, fit := range fits {
		fmt.Fprintf(w, "%s — %d measured pair(s)\n", fit.GameID, len(fit.Pairs))
		if len(fit.Pairs) == 0 {
			fmt.Fprintf(w, "  no two modes share a player with converged ratings\n\n")
			continue
		}

		fmt.Fprintf(w, "  %-18s %-18s %7s %6s %9s %9s %7s %7s %7s\n",
			"source", "target", "players", "r", "residual", "flat", "bias", "p50", "usable")
		for _, p := range fit.Pairs {
			usable := "no"
			if p.Usable() {
				usable = "yes"
			}
			fmt.Fprintf(w, "  %-18s %-18s %7d %6.3f %9.3f %9.3f %7.3f %7.3f %7s\n",
				p.SourceMode, p.TargetMode, p.PairedPlayers, p.Correlation,
				p.ResidualSD, p.FlatResidualSD, p.Bias, p.AbsErrorP50, usable)
		}

		// The seed a usable pair would actually serve, which is not readable
		// off the residual alone: the floor can lift it, and the correlation
		// and spreads decide how far a player moves from the mean.
		fmt.Fprintf(w, "  seed sigma floor: %.3f (flat prior: %.3f)\n\n",
			coldstart.SeedSigmaFloor, rating.UnratedSigma)
	}
}
