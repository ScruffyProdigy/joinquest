package graph

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/rating"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

const (
	demoModeKey = "duel"
	// demoOtherGameIDStr is seeded catalog game 002 (migrations 000003 / 000009),
	// retired but still a real games row — which is all a foreign key needs. It
	// stands in for "somewhere else in the catalog" in the scoping tests.
	demoOtherGameIDStr = "a1000000-0000-4000-8000-000000000002"
)

// seedPlayerRating writes one player's cached rating for a game and mode.
//
// SaveRatings clears the game/mode first, which is exactly what a test wants:
// each case starts from a known population rather than whatever a previous one
// left behind.
func seedPlayerRating(t *testing.T, env *queueIntegrationEnv, gameID uuid.UUID, modeKey string, userID uuid.UUID, v store.RatingValue) {
	t.Helper()
	err := env.Store.SaveRatings(
		context.Background(),
		gameID,
		modeKey,
		"test-engine@1",
		time.Now().UTC(),
		map[string]store.RatingValue{store.PlayerRatingKey(userID): v},
		nil,
	)
	if err != nil {
		t.Fatalf("SaveRatings: %v", err)
	}
}

type playerSkillResponse struct {
	Data struct {
		Player *struct {
			ID    string `json:"id"`
			Skill *struct {
				Rating      float64 `json:"rating"`
				Uncertainty float64 `json:"uncertainty"`
			} `json:"skill"`
		} `json:"player"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

const playerSkillQuery = `query Player($id: ID!, $modeKey: String!) {
	player(id: $id) {
		id
		skill(modeKey: $modeKey) { rating uncertainty }
	}
}`

func queryPlayerSkill(t *testing.T, env *queueIntegrationEnv, bearer, playerID, modeKey string) playerSkillResponse {
	t.Helper()
	body := postGraphQLWithBearer(t, env.Handler, bearer, playerSkillQuery, map[string]any{
		"id":      playerID,
		"modeKey": modeKey,
	})
	var resp playerSkillResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode player skill: %v body=%s", err, body)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("player skill errors: %+v", resp.Errors)
	}
	if resp.Data.Player == nil {
		t.Fatalf("expected a player, got %s", body)
	}
	return resp
}

// A game asking about a player it has actually rated gets that rating back, and
// one it has never seen gets the starting estimate rather than a null it would
// have to special-case.
func TestPlayerSkillOverServiceToken(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := t.Context()

	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "player-skill-pepper")

	rated := createTestUser(t, ctx, env, cleaner, "skill-rated-"+uuid.NewString()+"@example.com", "Rated Player")
	unrated := createTestUser(t, ctx, env, cleaner, "skill-unrated-"+uuid.NewString()+"@example.com", "Unrated Player")

	seedPlayerRating(t, env, store.DemoPrimaryGameID, demoModeKey, rated.ID, store.RatingValue{
		Mu: 31.5, Sigma: 2.25, MatchesPlayed: 42,
	})

	token := demoGameServiceToken(t)

	got := queryPlayerSkill(t, env, token, rated.ID.String(), demoModeKey)
	if got.Data.Player.Skill == nil {
		t.Fatal("a service-authenticated caller got no skill for a rated player")
	}
	if got.Data.Player.Skill.Rating != 31.5 {
		t.Errorf("rating = %v, want 31.5", got.Data.Player.Skill.Rating)
	}
	if got.Data.Player.Skill.Uncertainty != 2.25 {
		t.Errorf("uncertainty = %v, want 2.25", got.Data.Player.Skill.Uncertainty)
	}

	fresh := queryPlayerSkill(t, env, token, unrated.ID.String(), demoModeKey)
	if fresh.Data.Player.Skill == nil {
		t.Fatal("an unrated player reported no skill; the prior is meant to stand in so a game never special-cases null")
	}
	prior := rating.UnratedSkill()
	if fresh.Data.Player.Skill.Rating != prior.Rating {
		t.Errorf("unrated rating = %v, want the prior %v", fresh.Data.Player.Skill.Rating, prior.Rating)
	}
	if fresh.Data.Player.Skill.Uncertainty != prior.Uncertainty {
		t.Errorf("unrated uncertainty = %v, want the prior %v", fresh.Data.Player.Skill.Uncertainty, prior.Uncertainty)
	}
}

// A game's token reads its own catalog entry and no other. The player here is
// rated somewhere else entirely; the calling game must see a first-timer.
func TestPlayerSkillIsScopedToTheCallingGame(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := t.Context()

	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "player-skill-scope-pepper")

	user := createTestUser(t, ctx, env, cleaner, "skill-scope-"+uuid.NewString()+"@example.com", "Scoped Player")

	// Strong somewhere else in the catalog, and unrated in the calling game.
	seedPlayerRating(t, env, uuid.MustParse(demoOtherGameIDStr), demoModeKey, user.ID, store.RatingValue{
		Mu: 44.0, Sigma: 1.1, MatchesPlayed: 300,
	})
	if err := env.Store.ClearRatings(ctx, store.DemoPrimaryGameID, demoModeKey); err != nil {
		t.Fatalf("ClearRatings: %v", err)
	}

	got := queryPlayerSkill(t, env, demoGameServiceToken(t), user.ID.String(), demoModeKey)
	if got.Data.Player.Skill == nil {
		t.Fatal("expected the prior, got no skill at all")
	}
	if got.Data.Player.Skill.Rating != rating.UnratedMu || got.Data.Player.Skill.Uncertainty != rating.UnratedSigma {
		t.Errorf("a game read a rating from another game's catalog entry: %+v", *got.Data.Player.Skill)
	}
}

// A mode key the game does not declare answers null rather than a prior, so a
// typo reads as a mistake instead of as a plausible number.
func TestPlayerSkillUnknownModeIsNull(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := t.Context()

	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "player-skill-mode-pepper")

	user := createTestUser(t, ctx, env, cleaner, "skill-mode-"+uuid.NewString()+"@example.com", "Mode Player")

	got := queryPlayerSkill(t, env, demoGameServiceToken(t), user.ID.String(), "not-a-real-mode")
	if got.Data.Player.Skill != nil {
		t.Errorf("an undeclared mode returned %+v, want null", *got.Data.Player.Skill)
	}
}

// The two ways of reaching player(id) that are not a per-game service token.
//
// The dev case is the one worth stating plainly: with no service token
// configured, requireGameServiceAuth lets an unauthenticated caller through to
// player(id) entirely on purpose, so a local stack works. Skill does not ride
// that bypass — the whole claim behind this field is that only a game server
// sees it, and a claim with a configuration in which it is false is not a
// claim.
func TestPlayerSkillNeedsAPerGameServiceToken(t *testing.T) {
	t.Run("unauthenticated caller in development", func(t *testing.T) {
		env := newQueueIntegrationEnv(t)
		cleaner := env.newCleaner(t)
		ctx := t.Context()

		t.Setenv("LOBBY_GAME_SERVICE_TOKEN", "")
		t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "")

		user := createTestUser(t, ctx, env, cleaner, "skill-dev-"+uuid.NewString()+"@example.com", "Dev Player")
		seedPlayerRating(t, env, store.DemoPrimaryGameID, demoModeKey, user.ID, store.RatingValue{
			Mu: 31.5, Sigma: 2.25, MatchesPlayed: 42,
		})

		got := queryPlayerSkill(t, env, "", user.ID.String(), demoModeKey)
		if got.Data.Player.Skill != nil {
			t.Errorf("skill leaked to an unauthenticated caller: %+v", *got.Data.Player.Skill)
		}
	})

	t.Run("legacy global service token", func(t *testing.T) {
		env := newQueueIntegrationEnv(t)
		cleaner := env.newCleaner(t)
		ctx := t.Context()

		t.Setenv("LOBBY_GAME_SERVICE_TOKEN", "legacy-global-service-token")
		t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "")

		user := createTestUser(t, ctx, env, cleaner, "skill-legacy-"+uuid.NewString()+"@example.com", "Legacy Player")
		seedPlayerRating(t, env, store.DemoPrimaryGameID, demoModeKey, user.ID, store.RatingValue{
			Mu: 31.5, Sigma: 2.25, MatchesPlayed: 42,
		})

		// The legacy token authenticates but names no game, so there is no
		// answer to which catalog entry the rating should come from.
		got := queryPlayerSkill(t, env, "legacy-global-service-token", user.ID.String(), demoModeKey)
		if got.Data.Player.Skill != nil {
			t.Errorf("a token that names no game still read a rating: %+v", *got.Data.Player.Skill)
		}
	})
}

// PublicPlayer is not the service-only type its name suggests — it is also what
// backs a match roster the players themselves read. This walks that path as a
// player and asserts the field stays empty on it.
func TestPlayerFacingRosterCarriesNoSkill(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)

	match := seedActiveMatch(t, env, cleaner)
	seedPlayerRating(t, env, store.DemoPrimaryGameID, demoModeKey, match.userA.ID, store.RatingValue{
		Mu: 31.5, Sigma: 2.25, MatchesPlayed: 42,
	})

	query := `query Result($matchId: ID!) {
		matchResult(matchId: $matchId) {
			participants { user { id skill(modeKey: "` + demoModeKey + `") { rating } } }
		}
	}`
	body := postGraphQL(t, env.Handler, query, map[string]any{"matchId": match.sessionID.String()}, match.cookieA)

	var resp struct {
		Data struct {
			MatchResult *struct {
				Participants []struct {
					User struct {
						ID    string `json:"id"`
						Skill *struct {
							Rating float64 `json:"rating"`
						} `json:"skill"`
					} `json:"user"`
				} `json:"participants"`
			} `json:"matchResult"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode matchResult: %v body=%s", err, body)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("matchResult errors: %+v", resp.Errors)
	}
	if resp.Data.MatchResult == nil {
		t.Fatalf("participant got a null matchResult: %s", body)
	}
	if len(resp.Data.MatchResult.Participants) == 0 {
		t.Fatalf("no participants on the roster: %s", body)
	}
	for _, p := range resp.Data.MatchResult.Participants {
		if p.User.Skill != nil {
			t.Errorf("player-facing roster exposed skill for %s: %+v", p.User.ID, *p.User.Skill)
		}
	}
}

// Skill rides the provision push so a game can size the match before it starts.
func TestProvisionPayloadCarriesSeatSkill(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)

	// seedActiveMatch installs its own provisioner, so read it back off the
	// resolver rather than racing it with one of ours.
	seedActiveMatch(t, env, cleaner)
	provisioner, ok := env.resolver.GameProvisioner.(*syncProvisioner)
	if !ok {
		t.Fatalf("expected the seeded match to leave a syncProvisioner, got %T", env.resolver.GameProvisioner)
	}

	call := provisioner.lastCall()
	if len(call.Assignment.Seats) == 0 {
		t.Fatal("provision pushed no seats")
	}
	prior := rating.UnratedSkill()
	for _, seat := range call.Assignment.Seats {
		if seat.Skill == nil {
			t.Fatalf("seat %s was provisioned with no skill; a game reading the roster would have to make one up", seat.SeatKey)
		}
		if seat.Skill.Rating != prior.Rating || seat.Skill.Uncertainty != prior.Uncertainty {
			t.Errorf("seat %s: skill = %+v, want the prior for a first-time player", seat.SeatKey, *seat.Skill)
		}
	}
}

// attachSeatSkills is what fills the provision roster in. This drives it
// directly against the database so a rated seat and an unrated one can be
// checked side by side, which forming a real match for each would not make
// easy.
func TestAttachSeatSkillsFillsEverySeat(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := t.Context()

	rated := createTestUser(t, ctx, env, cleaner, "attach-rated-"+uuid.NewString()+"@example.com", "Attach Rated")
	unrated := createTestUser(t, ctx, env, cleaner, "attach-unrated-"+uuid.NewString()+"@example.com", "Attach Unrated")

	seedPlayerRating(t, env, store.DemoPrimaryGameID, demoModeKey, rated.ID, store.RatingValue{
		Mu: 18.25, Sigma: 4.5, MatchesPlayed: 7,
	})

	seats := []gameclient.AssignmentSeat{
		{SeatKey: "1", LobbyUserID: rated.ID.String()},
		{SeatKey: "2", LobbyUserID: unrated.ID.String()},
	}
	if err := attachSeatSkills(ctx, env.Store, store.DemoPrimaryGameID, demoModeKey, seats); err != nil {
		t.Fatalf("attachSeatSkills: %v", err)
	}

	if seats[0].Skill == nil || *seats[0].Skill != (gameclient.ProvisionSkill{Rating: 18.25, Uncertainty: 4.5}) {
		t.Errorf("rated seat = %+v, want the stored rating", seats[0].Skill)
	}
	prior := rating.UnratedSkill()
	want := gameclient.ProvisionSkill{Rating: prior.Rating, Uncertainty: prior.Uncertainty}
	if seats[1].Skill == nil || *seats[1].Skill != want {
		t.Errorf("unrated seat = %+v, want the prior", seats[1].Skill)
	}
}

// A service token is scoped to a game by construction, so a caller cannot ask
// about another one. This pins the layer under that: the id the resolver trusts
// comes from the token and nowhere else.
func TestServiceScopedGameIDRejectsNonGameCallers(t *testing.T) {
	if _, ok := serviceScopedGameID(context.Background()); ok {
		t.Error("a bare context passed as a game service caller")
	}

	playerCtx := auth.WithUserID(context.Background(), uuid.NewString())
	if _, ok := serviceScopedGameID(playerCtx); ok {
		t.Error("a player session passed as a game service caller")
	}

	unscoped := auth.WithGameServiceAuth(context.Background())
	if _, ok := serviceScopedGameID(unscoped); ok {
		t.Error("a service token naming no game passed as scoped")
	}

	scoped := auth.WithGameServiceGameID(auth.WithGameServiceAuth(context.Background()), store.DemoPrimaryGameIDStr)
	gameID, ok := serviceScopedGameID(scoped)
	if !ok || gameID != store.DemoPrimaryGameID {
		t.Errorf("scoped service token resolved to (%v, %v), want (%v, true)", gameID, ok, store.DemoPrimaryGameID)
	}
}
