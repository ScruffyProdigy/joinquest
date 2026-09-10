package graph

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestUpdatePlayerProfileAndPlayerLookup(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.test")
	t.Setenv("LOBBY_GAME_SERVICE_TOKEN", "test-game-service-token")

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "avatar-flow-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	profileQuery := `mutation Profile($displayName: String!, $avatarKey: ID!) {
		updatePlayerProfile(displayName: $displayName, avatarKey: $avatarKey) {
			displayName
			avatarKey
			avatarUrl
			avatarSource
		}
	}`
	profileBody := postGraphQL(t, env.Handler, profileQuery, map[string]any{
		"displayName": "River",
		"avatarKey":   "campfire",
	}, cookie)
	var profileResp struct {
		Data struct {
			UpdatePlayerProfile struct {
				DisplayName *string `json:"displayName"`
				AvatarKey   *string `json:"avatarKey"`
				AvatarURL   *string `json:"avatarUrl"`
			} `json:"updatePlayerProfile"`
		} `json:"data"`
	}
	if err := json.Unmarshal(profileBody, &profileResp); err != nil {
		t.Fatalf("decode profile: %v body=%s", err, profileBody)
	}
	profile := profileResp.Data.UpdatePlayerProfile
	if profile.DisplayName == nil || *profile.DisplayName != "River" {
		t.Fatalf("displayName: %+v", profile.DisplayName)
	}
	if profile.AvatarKey == nil || *profile.AvatarKey != "campfire" {
		t.Fatalf("avatarKey: %+v", profile.AvatarKey)
	}
	if profile.AvatarURL == nil || *profile.AvatarURL != "https://joinquest.test/avatars/campfire.png" {
		t.Fatalf("avatarUrl: %+v", profile.AvatarURL)
	}

	playerQuery := `query Player($id: ID!) { player(id: $id) { id avatarUrl avatarSource } }`
	playerBody := postGraphQLWithBearer(t, env.Handler, "test-game-service-token", playerQuery, map[string]any{"id": user.ID.String()})
	var playerResp struct {
		Data struct {
			Player *struct {
				AvatarURL *string `json:"avatarUrl"`
			} `json:"player"`
		} `json:"data"`
	}
	if err := json.Unmarshal(playerBody, &playerResp); err != nil {
		t.Fatalf("decode player: %v", err)
	}
	if playerResp.Data.Player == nil || playerResp.Data.Player.AvatarURL == nil {
		t.Fatalf("expected player avatar, got %+v", playerResp.Data.Player)
	}

	catalogQuery := `query { starterAvatars { key name slot imageUrl } }`
	catalogBody := postGraphQL(t, env.Handler, catalogQuery, nil, cookie)
	var catalogResp struct {
		Data struct {
			StarterAvatars []struct {
				Key string `json:"key"`
			} `json:"starterAvatars"`
		} `json:"data"`
	}
	if err := json.Unmarshal(catalogBody, &catalogResp); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(catalogResp.Data.StarterAvatars) != 18 {
		t.Fatalf("expected 18 starter avatars, got %d", len(catalogResp.Data.StarterAvatars))
	}
}

func TestUpdatePlayerProfileNameOnlyPreservesSpiritAnimal(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.test")

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "spirit-name-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	spiritURL := "https://joinquest.test/avatars/spirit/fox.png"
	_, err = env.DB.ExecContext(ctx, `
		UPDATE users
		SET avatar_url = $2, avatar_key = NULL, avatar_source = $3
		WHERE id = $1
	`, user.ID, spiritURL, store.SourceSpiritAnimal)
	if err != nil {
		t.Fatalf("set spirit animal avatar: %v", err)
	}
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	profileQuery := `mutation Profile($displayName: String!) {
		updatePlayerProfile(displayName: $displayName) {
			displayName
			avatarKey
			avatarUrl
			avatarSource
		}
	}`
	profileBody := postGraphQL(t, env.Handler, profileQuery, map[string]any{
		"displayName": "River",
	}, cookie)
	var profileResp struct {
		Data struct {
			UpdatePlayerProfile struct {
				DisplayName  *string `json:"displayName"`
				AvatarKey    *string `json:"avatarKey"`
				AvatarURL    *string `json:"avatarUrl"`
				AvatarSource *string `json:"avatarSource"`
			} `json:"updatePlayerProfile"`
		} `json:"data"`
	}
	if err := json.Unmarshal(profileBody, &profileResp); err != nil {
		t.Fatalf("decode profile: %v body=%s", err, profileBody)
	}
	profile := profileResp.Data.UpdatePlayerProfile
	if profile.DisplayName == nil || *profile.DisplayName != "River" {
		t.Fatalf("displayName: %+v", profile.DisplayName)
	}
	if profile.AvatarKey != nil {
		t.Fatalf("avatarKey should stay nil, got %+v", profile.AvatarKey)
	}
	if profile.AvatarURL == nil || *profile.AvatarURL != spiritURL {
		t.Fatalf("avatarUrl: %+v", profile.AvatarURL)
	}
	if profile.AvatarSource == nil || *profile.AvatarSource != "SPIRIT_ANIMAL" {
		t.Fatalf("avatarSource: %+v", profile.AvatarSource)
	}
}

func TestMyRoomIncludesMemberAvatars(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.test")

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "avatar-room-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	if _, err := env.Store.UpdateUserProfile(ctx, user.ID, "Pat", "compass", "https://joinquest.test"); err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	postGraphQL(t, env.Handler, `mutation { createRoom { inviteCode } }`, nil, cookie)

	myRoomBody := postGraphQL(t, env.Handler, `query {
		myRoom {
			members { user { id avatarUrl avatarKey } away }
			host { id avatarUrl }
		}
	}`, nil, cookie)
	var roomResp struct {
		Data struct {
			MyRoom *struct {
				Members []struct {
					User struct {
						AvatarURL *string `json:"avatarUrl"`
					} `json:"user"`
				} `json:"members"`
			} `json:"myRoom"`
		} `json:"data"`
	}
	if err := json.Unmarshal(myRoomBody, &roomResp); err != nil {
		t.Fatalf("decode myRoom: %v body=%s", err, myRoomBody)
	}
	if roomResp.Data.MyRoom == nil || len(roomResp.Data.MyRoom.Members) == 0 {
		t.Fatal("expected room members")
	}
	if roomResp.Data.MyRoom.Members[0].User.AvatarURL == nil {
		t.Fatal("expected member avatarUrl")
	}
}

// A guest coming through the first-entry avatar picker saves a generated name and
// one of the guest-tier sigils in a single call.
func TestUpdatePlayerProfileAcceptsGuestSigil(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.test")

	guest, err := env.Store.CreateGuestUser(ctx)
	if err != nil {
		t.Fatalf("CreateGuestUser: %v", err)
	}
	cleaner.TrackUser(guest.ID)
	_, cookie := createTestUserSessionForUser(t, env, guest.ID)

	query := `mutation Profile($displayName: String!, $avatarKey: ID!) {
		updatePlayerProfile(displayName: $displayName, avatarKey: $avatarKey) {
			displayName
			avatarKey
			avatarUrl
			avatarSource
		}
	}`
	body := postGraphQL(t, env.Handler, query, map[string]any{
		"displayName": "FrostFox4827",
		"avatarKey":   "sigil-canine-frost",
	}, cookie)

	var resp struct {
		Data struct {
			UpdatePlayerProfile struct {
				DisplayName  *string `json:"displayName"`
				AvatarKey    *string `json:"avatarKey"`
				AvatarURL    *string `json:"avatarUrl"`
				AvatarSource *string `json:"avatarSource"`
			} `json:"updatePlayerProfile"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode profile: %v body=%s", err, body)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("unexpected errors: %+v", resp.Errors)
	}

	profile := resp.Data.UpdatePlayerProfile
	if profile.DisplayName == nil || *profile.DisplayName != "FrostFox4827" {
		t.Fatalf("displayName: %+v", profile.DisplayName)
	}
	if profile.AvatarKey == nil || *profile.AvatarKey != "sigil-canine-frost" {
		t.Fatalf("avatarKey: %+v", profile.AvatarKey)
	}
	// The key keeps the name it was saved under, but the URL it resolves to is the
	// rendered one: "frost" is the hex it always stood for, and a key with no
	// expression gets the plain face.
	if profile.AvatarURL == nil || *profile.AvatarURL != "https://joinquest.test/avatars/sigils/canine-0284c7-wide.svg" {
		t.Fatalf("avatarUrl: %+v", profile.AvatarURL)
	}
	if profile.AvatarSource == nil || *profile.AvatarSource != "SIGIL" {
		t.Fatalf("avatarSource: %+v", profile.AvatarSource)
	}
}
