package graph

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/playhub/graph/model"
	"github.com/scruffyprodigy/playhub/internal/catalogstats"
	"github.com/scruffyprodigy/playhub/internal/store"
	"github.com/scruffyprodigy/playhub/internal/testdb"
)

func newLiveCountsResolver(t *testing.T) (*Resolver, *store.Store) {
	t.Helper()

	db, err := sql.Open("postgres", testdb.RequireURL(t))
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st := store.New(db)
	return &Resolver{Store: st}, st
}

func TestGamePlayerActivityResolver(t *testing.T) {
	resolver, st := newLiveCountsResolver(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, err := st.InsertTestGame(ctx, "Player Activity Game")
	if err != nil {
		t.Fatalf("InsertTestGame: %v", err)
	}
	cleaner.TrackGame(game.ID)

	player, err := st.CreateUser(ctx, store.CreateUserParams{
		Email:       "activity-" + uuid.NewString() + "@example.com",
		DisplayName: "Activity Player",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(player.ID)

	session, err := st.CreateSession(ctx, game.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := st.AddSessionParticipant(ctx, session.ID, player.ID, "player"); err != nil {
		t.Fatalf("AddSessionParticipant: %v", err)
	}

	obj := &model.Game{ID: game.ID.String()}

	// With no cache configured the resolver falls back to querying the store directly.
	activity, err := resolver.Game().PlayerActivity(ctx, obj)
	if err != nil {
		t.Fatalf("PlayerActivity (no cache): %v", err)
	}
	if activity.Playing != 1 || activity.Queued != 0 {
		t.Fatalf("uncached activity = %+v, want {Playing:1 Queued:0}", activity)
	}

	// With a cache the same numbers come back, now served from the shared snapshot.
	resolver.LiveCountsCache = catalogstats.NewCache(liveCountsSource(st), time.Minute)
	cached, err := resolver.Game().PlayerActivity(ctx, obj)
	if err != nil {
		t.Fatalf("PlayerActivity (cached): %v", err)
	}
	if cached.Playing != 1 || cached.Queued != 0 {
		t.Fatalf("cached activity = %+v, want {Playing:1 Queued:0}", cached)
	}
}

func TestGamePlayerActivityRejectsABadGameID(t *testing.T) {
	resolver, _ := newLiveCountsResolver(t)

	if _, err := resolver.Game().PlayerActivity(context.Background(), &model.Game{ID: "not-a-uuid"}); err == nil {
		t.Fatal("expected an error for a malformed game id")
	}
}
