package graph

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/scruffyprodigy/playhub/internal/gameclient/testutil"
	"github.com/scruffyprodigy/playhub/internal/store"
)

const fixtureGameID = "b1000000-0000-4000-8000-000000000001"

// eligibilityFixtureServerWithSuffix wraps testutil.EligibilityFixtureHandler so a test can pick
// which locked/unlocked scenario it exercises. The resolver always forwards the real authenticated
// player's UUID as lobbyUserId, and no valid UUID string can end in "-locked"/"-unlocked" (the
// convention the fixture handler keys its canned responses on), so the caller-supplied suffix is
// spliced onto the path here rather than relying on the player id itself to carry it.
func eligibilityFixtureServerWithSuffix(suffix string) *httptest.Server {
	inner := testutil.EligibilityFixtureHandler()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/players/") && strings.HasSuffix(r.URL.Path, "/mode-eligibility") {
			r.URL.Path = strings.TrimSuffix(r.URL.Path, "/mode-eligibility") + suffix + "/mode-eligibility"
		}
		inner.ServeHTTP(w, r)
	}))
}

func pointFixtureGameAt(t *testing.T, env *queueIntegrationEnv, apiBaseURL string) {
	t.Helper()
	if _, err := env.DB.Exec(`UPDATE games SET api_base_url = $1 WHERE id = $2`, apiBaseURL, fixtureGameID); err != nil {
		t.Fatalf("point fixture game at test server: %v", err)
	}
	t.Cleanup(func() {
		_, _ = env.DB.Exec(`UPDATE games SET api_base_url = 'http://localhost:9400' WHERE id = $1`, fixtureGameID)
	})
}

func TestGameModeEligibilityLockedLeaf(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := t.Context()
	cleaner := env.newCleaner(t)

	srv := httptest.NewServer(testutil.EligibilityFixtureHandler())
	defer srv.Close()
	pointFixtureGameAt(t, env, srv.URL)

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{Email: "player-legendary-locked@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	var resp struct {
		Game struct {
			Modes []struct {
				ModeKey     string `json:"modeKey"`
				Eligibility struct {
					Accessible bool    `json:"accessible"`
					Reason     *string `json:"reason"`
				} `json:"eligibility"`
			} `json:"modes"`
		} `json:"game"`
	}
	err = env.Client.Post(`
		query($gameId: ID!, $playerId: ID!) {
			game(id: $gameId) {
				modes {
					modeKey
					eligibility(playerId: $playerId) { accessible reason }
				}
			}
		}
	`, &resp, client.AddCookie(cookie), client.Var("gameId", fixtureGameID), client.Var("playerId", user.ID.String()))
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	found := false
	for _, mode := range resp.Game.Modes {
		if mode.ModeKey == "legendary" {
			found = true
			if mode.Eligibility.Accessible {
				t.Fatal("expected legendary to be locked")
			}
			if mode.Eligibility.Reason == nil || *mode.Eligibility.Reason != "Complete 50 Ranked matches to unlock." {
				t.Fatalf("unexpected reason: %+v", mode.Eligibility.Reason)
			}
		}
	}
	if !found {
		t.Fatal("legendary mode not found in response")
	}
}

func TestGameModeEligibilityCompoundGroupUnlocked(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := t.Context()
	cleaner := env.newCleaner(t)

	srv := eligibilityFixtureServerWithSuffix("-unlocked")
	defer srv.Close()
	pointFixtureGameAt(t, env, srv.URL)

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{Email: "player-commander-unlocked@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	var resp struct {
		Game struct {
			Modes []struct {
				ModeKey     string `json:"modeKey"`
				Eligibility struct {
					Accessible  bool `json:"accessible"`
					Requirement struct {
						Typename string `json:"__typename"`
						Operator string `json:"operator"`
						Children []struct {
							Current int `json:"current"`
							Target  int `json:"target"`
						} `json:"children"`
					} `json:"requirement"`
				} `json:"eligibility"`
			} `json:"modes"`
		} `json:"game"`
	}
	err = env.Client.Post(`
		query($gameId: ID!, $playerId: ID!) {
			game(id: $gameId) {
				modes {
					modeKey
					eligibility(playerId: $playerId) {
						accessible
						requirement {
							__typename
							... on RequirementGroup {
								operator
								children { ... on RequirementLeaf { current target } }
							}
						}
					}
				}
			}
		}
	`, &resp, client.AddCookie(cookie), client.Var("gameId", fixtureGameID), client.Var("playerId", user.ID.String()))
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	for _, mode := range resp.Game.Modes {
		if mode.ModeKey != "commander" {
			continue
		}
		if !mode.Eligibility.Accessible {
			t.Fatal("expected commander to be unlocked for -unlocked player")
		}
		if mode.Eligibility.Requirement.Operator != "ALL" {
			t.Fatalf("expected ALL operator, got %q", mode.Eligibility.Requirement.Operator)
		}
		if len(mode.Eligibility.Requirement.Children) != 2 {
			t.Fatalf("expected 2 children, got %d", len(mode.Eligibility.Requirement.Children))
		}
	}
}

func TestGameModeEligibilityRejectsQueryingAnotherPlayer(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	ctx := t.Context()
	cleaner := env.newCleaner(t)

	srv := httptest.NewServer(testutil.EligibilityFixtureHandler())
	defer srv.Close()
	pointFixtureGameAt(t, env, srv.URL)

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{Email: "player-a@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	other, err := env.Store.CreateUser(ctx, store.CreateUserParams{Email: "player-b@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(other.ID)
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	var resp struct{}
	err = env.Client.Post(`
		query($gameId: ID!, $playerId: ID!) {
			game(id: $gameId) { modes { eligibility(playerId: $playerId) { accessible } } }
		}
	`, &resp, client.AddCookie(cookie), client.Var("gameId", fixtureGameID), client.Var("playerId", other.ID.String()))
	if err == nil {
		t.Fatal("expected an authorization error when querying another player's eligibility")
	}
}
