package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/scruffyprodigy/joinquest/database"
	"github.com/scruffyprodigy/joinquest/graph"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/avatars"
	"github.com/scruffyprodigy/joinquest/internal/formingworker"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/ratingworker"
	"github.com/scruffyprodigy/joinquest/internal/spiritanimal"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func main() {
	if err := database.InitWithMigrations(); err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer database.Close()

	dataStore := store.New(database.GetDB())
	if err := dataStore.Ping(context.Background()); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	signer, err := auth.LoadSignerFromEnv()
	if err != nil {
		log.Fatalf("JWT signer initialization failed: %v", err)
	}

	authService, err := auth.NewService(dataStore, signer)
	if err != nil {
		log.Fatalf("Auth service initialization failed: %v", err)
	}

	broker, err := pubsub.NewFromEnv()
	if err != nil {
		log.Fatalf("PubSub initialization failed: %v", err)
	}
	defer broker.Close()

	if strings.TrimSpace(os.Getenv("REDIS_URL")) == "" {
		log.Println("pubsub: REDIS_URL not set, using in-memory broker (single instance only)")
	} else {
		log.Println("pubsub: connected to Redis")
	}
	if pubsub.DebugEnabled() {
		log.Println("pubsub: LOBBY_PUBSUB_DEBUG enabled — queue publish/subscribe tracing active")
	}

	resolver := graph.NewResolver(dataStore, authService, broker)
	resolver.SpiritAnimal = spiritanimal.NewRunnerFromEnv(dataStore, auth.LobbyPublicURL())
	go resolver.SpiritAnimal.ResumeAllStale(context.Background())

	// A fresh process holds no sockets, so any count left in user_presence belongs to
	// a previous life of this server. Zeroing them is pessimistic in the safe
	// direction: Postgres has no TTL, so without this a pod that died without running
	// its defers would leave absent players reading as present indefinitely, and
	// genuinely-live players re-increment within seconds when their client reconnects.
	if cleared, err := dataStore.ResetPresenceOnBoot(context.Background()); err != nil {
		log.Printf("presence: boot reset failed: %v", err)
	} else if cleared > 0 {
		log.Printf("presence: boot reset cleared %d stale rows", cleared)
	}
	// Two windows off one socket edge, on deliberately different clocks: a waiting
	// player keeps their queue place for 90s, a room member keeps their seat in the
	// room for 30s. See each constant for why they are not the same number.
	resolver.Presence = graph.NewPresenceTracker(
		dataStore,
		broker,
		store.DefaultQueueDisconnectGrace,
		resolver.OnGraceExpired,
	).WithExpiry(
		store.DefaultRoomDisconnectGrace,
		resolver.OnRoomGraceExpired,
	)

	formingTick := 30 * time.Second
	if v := strings.TrimSpace(os.Getenv("FORMING_RECONCILE_INTERVAL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			formingTick = d
		}
	}
	resolver.FormingWorker = formingworker.New(dataStore, resolver.HandleFormingReconciled, 25*time.Millisecond, formingTick)
	resolver.FormingWorker.SetProvisionHook(resolver.HandleUnprovisionedSession)
	go resolver.FormingWorker.Start(context.Background())

	ratingTick := 5 * time.Second
	if v := strings.TrimSpace(os.Getenv("RATING_REPLAY_INTERVAL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			ratingTick = d
		}
	}
	ratingEngine, err := rating.NewWengLin("plackett-luce")
	if err != nil {
		log.Fatalf("rating engine: %v", err)
	}
	resolver.RatingWorker = ratingworker.New(func() ratingworker.Replayer {
		return rating.NewReplayer(ratingEngine, dataStore.RatingSource())
	}, ratingTick)
	resolver.RatingWorker.SetSweeper(dataStore, 10*time.Minute)
	go resolver.RatingWorker.Start(context.Background())

	mux := http.NewServeMux()

	gql := graph.NewGraphQLServer(signer, dataStore, resolver)

	mux.Handle("/graphql", auth.Middleware(signer, dataStore, gql))
	spiritAvatarDir := strings.TrimSpace(os.Getenv("SPIRIT_AVATAR_STORAGE_DIR"))
	if spiritAvatarDir == "" {
		spiritAvatarDir = "data/spirit-avatars"
	}
	mux.Handle("/spirit-avatars/", http.StripPrefix("/spirit-avatars/", http.FileServer(http.Dir(filepath.Clean(spiritAvatarDir)))))
	mux.Handle("/avatars/sigils/", avatars.SigilHandler())
	if !auth.IsProductionEnv() {
		mux.Handle("/", playground.Handler("GraphQL", "/graphql"))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			http.NotFound(w, nil)
		})
	}

	mux.Handle("/.well-known/jwks.json", signer.JWKSHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	auth.RegisterOAuthRoutes(mux, authService, signer, dataStore)

	handler := auth.CORSMiddleware(mux)

	srv := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Println("backend listening :8080")
	log.Fatal(srv.ListenAndServe())
}
