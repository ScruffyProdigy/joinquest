package graph

import (
	"context"
	"database/sql"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/scruffyprodigy/joinquest/graph/generated"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
	"github.com/scruffyprodigy/joinquest/internal/testdb"
)

func newGamesGraphQLTestClient(t *testing.T) (*client.Client, *store.Store) {
	t.Helper()

	databaseURL := testdb.RequireURL(t)

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st := store.New(db)
	signer, err := auth.LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	authService, err := auth.NewService(st, signer)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	resolver := NewResolver(st, authService, pubsub.NewMemory())
	gql := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	return client.New(gql), st
}

func TestGamesGraphQLListsCatalogGames(t *testing.T) {
	c, _ := newGamesGraphQLTestClient(t)

	var resp struct {
		Games []struct {
			ID   string
			Name string
		} `json:"games"`
	}
	if err := c.Post(`query {
		games {
			id
			name
		}
	}`, &resp); err != nil {
		t.Fatalf("games query failed: %v", err)
	}

	if len(resp.Games) < 1 {
		t.Fatalf("expected at least 1 catalog game, got %d", len(resp.Games))
	}

	names := map[string]bool{}
	for _, game := range resp.Games {
		names[game.Name] = true
	}
	// Seed row ...0001 is Word Hunt (JQ-203); before that fix it wore RPSLR's
	// name, so both catalog cards read "Rock Paper Scissors Lizard Robot". The
	// RPSLR row is asserted by id below rather than here, because this listing is
	// paginated and integration fixtures crowd the first page.
	if !names["Word Hunt"] {
		t.Fatalf("expected Word Hunt catalog game in results, got %+v", resp.Games)
	}
	if names["Party Lobby"] {
		t.Fatal("expected retired Party Lobby name to be gone")
	}
}

func TestGameGraphQLReturnsDemoGameByID(t *testing.T) {
	c, _ := newGamesGraphQLTestClient(t)

	var resp struct {
		Game struct {
			ID             string
			Name           string
			ActiveSessions []struct {
				ID string
			} `json:"activeSessions"`
		} `json:"game"`
	}
	if err := c.Post(`query Game($id: ID!) {
		game(id: $id) {
			id
			name
			activeSessions {
				id
			}
		}
	}`, &resp, client.Var("id", "a1000000-0000-4000-8000-000000000001")); err != nil {
		t.Fatalf("game query failed: %v", err)
	}

	if resp.Game.Name != "Word Hunt" {
		t.Fatalf("expected Word Hunt, got %q", resp.Game.Name)
	}
}

// JQ-203: the two seed rows must answer with their own names. Both used to
// return "Rock Paper Scissors Lizard Robot".
func TestGameGraphQLSeedRowsHaveDistinctNames(t *testing.T) {
	c, _ := newGamesGraphQLTestClient(t)

	want := map[string]string{
		"a1000000-0000-4000-8000-000000000001": "Word Hunt",
		"a1000000-0000-4000-8000-000000000002": "Rock Paper Scissors Lizard Robot",
	}

	for id, name := range want {
		var resp struct {
			Game struct {
				Name string
				Slug string
			} `json:"game"`
		}
		if err := c.Post(`query Game($id: ID!) {
			game(id: $id) {
				name
				slug
			}
		}`, &resp, client.Var("id", id)); err != nil {
			t.Fatalf("game query for %s failed: %v", id, err)
		}
		if resp.Game.Name != name {
			t.Errorf("game %s: name = %q, want %q", id, resp.Game.Name, name)
		}
	}
}

func TestGameGraphQLReturnsNotFoundForMissingGame(t *testing.T) {
	c, _ := newGamesGraphQLTestClient(t)

	err := c.Post(`query Game($id: ID!) {
		game(id: $id) {
			id
		}
	}`, &struct{}{}, client.Var("id", uuid.NewString()))
	if err == nil {
		t.Fatal("expected game query to fail for missing game")
	}
}

func TestSessionGraphQLLoadsNestedGameAndPlayers(t *testing.T) {
	c, st := newGamesGraphQLTestClient(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, err := st.InsertTestGame(ctx, "GraphQL Session Test "+uuid.NewString())
	if err != nil {
		t.Fatalf("InsertTestGame failed: %v", err)
	}
	cleaner.TrackGame(game.ID)

	session, err := st.CreateSession(ctx, game.ID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	user, err := st.CreateUser(ctx, store.CreateUserParams{
		Email: "session-test-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	cleaner.TrackUser(user.ID)

	if err := st.AddSessionParticipant(ctx, session.ID, user.ID, "player"); err != nil {
		t.Fatalf("AddSessionParticipant failed: %v", err)
	}

	var resp struct {
		Session struct {
			ID     string
			Status string
			Game   struct {
				ID   string
				Name string
			}
			Players []struct {
				Email *string `json:"email"`
			}
		} `json:"session"`
	}
	if err := c.Post(`query Session($id: ID!) {
		session(id: $id) {
			id
			status
			game {
				id
				name
			}
			players {
				email
			}
		}
	}`, &resp, client.Var("id", session.ID.String())); err != nil {
		t.Fatalf("session query failed: %v", err)
	}

	if resp.Session.Game.ID != game.ID.String() {
		t.Fatalf("expected session game id %s, got %s", game.ID, resp.Session.Game.ID)
	}
	if len(resp.Session.Players) != 1 {
		t.Fatalf("expected 1 player, got %d", len(resp.Session.Players))
	}
}
