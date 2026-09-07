// One-off helper: refresh a catalog game's cached manifest from its api_base_url.
// Usage: DATABASE_URL=... go run ./cmd/syncmanifest <game-id>
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if len(os.Args) < 2 {
		log.Fatal("usage: syncmanifest <game-id>")
	}
	gameID, err := uuid.Parse(os.Args[1])
	if err != nil {
		log.Fatalf("invalid game id: %v", err)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	st := store.New(db)
	ctx := context.Background()
	game, err := st.GetGameByID(ctx, gameID)
	if err != nil {
		log.Fatalf("get game: %v", err)
	}
	if game.APIBaseURL == nil || *game.APIBaseURL == "" {
		log.Fatal("game has no api_base_url")
	}

	manifest, err := gameclient.NewManifestFetcher().Fetch(ctx, *game.APIBaseURL, "")
	if err != nil {
		log.Fatalf("fetch manifest: %v", err)
	}

	result, err := st.ApplyGameManifest(ctx, gameID, manifest)
	if err != nil {
		log.Fatalf("apply manifest: %v", err)
	}

	fmt.Printf("synced %q changed=%v kicked=%d\n", game.Name, result.Changed, len(result.Kicked))
}
