package graph

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/store"
)

// testAvatarKey is a real starter avatar, so fixtures that need a full identity
// get a face the same way a player does.
const testAvatarKey = "storm"

// createGuestSession builds a signed session for a guest at whatever stage of
// the identity prompt the caller wants: no name and no face, or a name only.
func createGuestSession(t *testing.T, ctx context.Context, env *queueIntegrationEnv, cleaner *store.TestCleaner, displayName, avatarKey string) (uuid.UUID, *http.Cookie) {
	t.Helper()

	user, err := env.Store.CreateGuestUser(ctx)
	if err != nil {
		t.Fatalf("CreateGuestUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	if displayName != "" || avatarKey != "" {
		if user, err = env.Store.UpdateUserProfile(ctx, user.ID, displayName, avatarKey, "http://localhost:5173"); err != nil {
			t.Fatalf("UpdateUserProfile: %v", err)
		}
	}
	if user.HasChosenIdentity() {
		t.Fatalf("fixture was meant to be short of a full identity, got name %q avatar %v", user.ChosenDisplayName(), user.HasChosenAvatar())
	}

	_, cookie := createTestUserSessionForUser(t, env, user.ID)
	return user.ID, cookie
}

// countRowsForUser counts a user's rows in one table, so a rejected mutation can
// be shown to have written nothing.
func countRowsForUser(t *testing.T, env *queueIntegrationEnv, table, column string, userID uuid.UUID) int {
	t.Helper()

	var count int
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + column + " = $1"
	if err := env.DB.QueryRowContext(context.Background(), query, userID).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func assertNoPlayEntryState(t *testing.T, env *queueIntegrationEnv, userID uuid.UUID) {
	t.Helper()

	for _, check := range []struct {
		table  string
		column string
	}{
		{"game_queues", "user_id"},
		{"rooms", "host_user_id"},
		{"room_members", "user_id"},
		{"table_seats", "user_id"},
	} {
		if got := countRowsForUser(t, env, check.table, check.column, userID); got != 0 {
			t.Fatalf("rejected mutation wrote %d row(s) to %s", got, check.table)
		}
	}
}

type playEntryCase struct {
	name  string
	query string
	vars  []client.Option
}

// playEntryMutations is every mutation that puts a player into play. The guard
// runs before any of the ids below are looked up, so unused ones are fine.
func playEntryMutations(someID string) []playEntryCase {
	return []playEntryCase{
		{
			name:  "joinQueue",
			query: `mutation Join($id: ID!) { joinQueue(queueId: $id) { queued } }`,
			vars:  []client.Option{client.Var("id", demoDefaultQueueID)},
		},
		{
			name:  "createRoom",
			query: `mutation { createRoom { id } }`,
		},
		{
			name:  "joinRoom",
			query: `mutation Join($code: String!) { joinRoom(inviteCode: $code) { id } }`,
			vars:  []client.Option{client.Var("code", "ABCDEF")},
		},
		{
			name:  "createPrivateTable",
			query: `mutation Create($gameId: ID!, $modeId: ID!) { createPrivateTable(gameId: $gameId, modeId: $modeId) { id } }`,
			vars:  []client.Option{client.Var("gameId", someID), client.Var("modeId", someID)},
		},
		{
			name:  "createTable",
			query: `mutation Create($roomId: ID!, $gameId: ID!, $modeId: ID!) { createTable(roomId: $roomId, gameId: $gameId, modeId: $modeId) { id } }`,
			vars:  []client.Option{client.Var("roomId", someID), client.Var("gameId", someID), client.Var("modeId", someID)},
		},
		{
			name:  "sitAtTable",
			query: `mutation Sit($tableId: ID!, $seatKey: String!) { sitAtTable(tableId: $tableId, seatKey: $seatKey) { id } }`,
			vars:  []client.Option{client.Var("tableId", someID), client.Var("seatKey", "seat-1")},
		},
		{
			name:  "startTable",
			query: `mutation Start($tableId: ID!) { startTable(tableId: $tableId) { queued } }`,
			vars:  []client.Option{client.Var("tableId", someID)},
		},
		{
			name:  "startTableBackfill",
			query: `mutation Backfill($tableId: ID!, $queueId: ID!) { startTableBackfill(tableId: $tableId, queueId: $queueId) { queued } }`,
			vars:  []client.Option{client.Var("tableId", someID), client.Var("queueId", demoDefaultQueueID)},
		},
	}
}

// TestPlayEntryMutationsRejectIncompleteIdentity covers every mutation that puts
// a player into play, against both halves of an unfinished identity. A session
// is not enough, and neither is a name on its own.
func TestPlayEntryMutationsRejectIncompleteIdentity(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")

	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	someID := uuid.NewString()

	for _, fixture := range []struct {
		label       string
		displayName string
		avatarKey   string
	}{
		{label: "no name and no avatar"},
		{label: "name but no avatar", displayName: "Half Done"},
	} {
		t.Run(fixture.label, func(t *testing.T) {
			userID, cookie := createGuestSession(t, ctx, env, cleaner, fixture.displayName, fixture.avatarKey)

			for _, tc := range playEntryMutations(someID) {
				t.Run(tc.name, func(t *testing.T) {
					opts := append([]client.Option{client.AddCookie(cookie)}, tc.vars...)

					var resp map[string]any
					err := env.Client.Post(tc.query, &resp, opts...)
					if err == nil {
						t.Fatalf("%s accepted a guest with %s", tc.name, fixture.label)
					}
					if !strings.Contains(err.Error(), "identity required") {
						t.Fatalf("%s: want an identity error, got %v", tc.name, err)
					}
					// A plain auth failure would send the player to sign-in
					// instead of the identity prompt, so the two must stay
					// distinguishable.
					if strings.Contains(err.Error(), "authentication required") {
						t.Fatalf("%s: identity error must not read as an auth error: %v", tc.name, err)
					}
					assertNoPlayEntryState(t, env, userID)
				})
			}
		})
	}
}

// TestPlayEntryMutationsAcceptIdentifiedGuest is the other half: the guard wants
// an identity, not an account, so a guest passes as soon as they finish picking
// a name and a face.
func TestPlayEntryMutationsAcceptIdentifiedGuest(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")

	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	userID, cookie := createGuestSession(t, ctx, env, cleaner, "", "")

	var rejected map[string]any
	if err := env.Client.Post(`mutation { createRoom { id } }`, &rejected, client.AddCookie(cookie)); err == nil {
		t.Fatal("createRoom accepted a guest with no identity")
	}

	var profileResp struct {
		UpdatePlayerProfile struct {
			DisplayName string
			AvatarURL   string `json:"avatarUrl"`
			IsGuest     bool
		} `json:"updatePlayerProfile"`
	}
	if err := env.Client.Post(`mutation Identify($name: String!, $avatarKey: ID!) {
		updatePlayerProfile(displayName: $name, avatarKey: $avatarKey) { displayName avatarUrl isGuest }
	}`, &profileResp, client.AddCookie(cookie),
		client.Var("name", "Named Guest"),
		client.Var("avatarKey", testAvatarKey),
	); err != nil {
		t.Fatalf("updatePlayerProfile: %v", err)
	}
	if profileResp.UpdatePlayerProfile.DisplayName != "Named Guest" {
		t.Fatalf("display name: %q", profileResp.UpdatePlayerProfile.DisplayName)
	}
	if profileResp.UpdatePlayerProfile.AvatarURL == "" {
		t.Fatal("expected an avatar url")
	}
	if !profileResp.UpdatePlayerProfile.IsGuest {
		t.Fatal("expected the caller to still be a guest — the guard wants an identity, not an account")
	}

	var roomResp struct {
		CreateRoom struct {
			ID string
		} `json:"createRoom"`
	}
	if err := env.Client.Post(`mutation { createRoom { id } }`, &roomResp, client.AddCookie(cookie)); err != nil {
		t.Fatalf("createRoom after identifying: %v", err)
	}
	if roomResp.CreateRoom.ID == "" {
		t.Fatal("expected a room id")
	}

	var queueResp struct {
		JoinQueue struct {
			Queued bool
		} `json:"joinQueue"`
	}
	if err := env.Client.Post(`mutation Join($id: ID!) { joinQueue(queueId: $id) { queued } }`,
		&queueResp, client.AddCookie(cookie), client.Var("id", demoDefaultQueueID)); err != nil {
		t.Fatalf("joinQueue after identifying: %v", err)
	}
	if !queueResp.JoinQueue.Queued {
		t.Fatal("expected the identified guest to be queued")
	}

	if got := countRowsForUser(t, env, "rooms", "host_user_id", userID); got != 1 {
		t.Fatalf("expected 1 room for the identified guest, got %d", got)
	}
}
