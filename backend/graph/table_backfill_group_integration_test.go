package graph

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/gameclient"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// reconcileQueue runs the forming step the worker would have run.
func reconcileQueue(t *testing.T, env *queueIntegrationEnv, queueID string) {
	t.Helper()
	id, err := uuid.Parse(queueID)
	if err != nil {
		t.Fatalf("parse queue id: %v", err)
	}
	if _, err := env.Store.ReconcileFormingModeQueue(context.Background(), id); err != nil {
		t.Fatalf("ReconcileFormingModeQueue: %v", err)
	}
}

// registerThreeSeatGame gives the test a mode with room for three, so two players can sit
// without filling the table — a full one now starts itself (JQ-137).
func registerThreeSeatGame(t *testing.T, env *queueIntegrationEnv, cleaner *store.TestCleaner) (gameID, modeID string) {
	t.Helper()
	ctx := context.Background()
	slug := "graph-trio-" + uuid.NewString()
	result, err := env.Store.RegisterGame(ctx, store.RegisterGameParams{
		Slug:       slug,
		IconURL:    "/games/default.svg",
		HeroURL:    "/games/default-hero.svg",
		APIBaseURL: "https://api.example.com/" + slug,
	}, &gameclient.Manifest{
		Modes: []gameclient.ModeManifest{{
			Key:          "trio",
			DisplayName:  "Trio",
			SeatTemplate: json.RawMessage(`{"count":3}`),
		}},
		Status:     gameclient.StatusResponse{Game: "Trio", Version: "1.0.0"},
		ETag:       `"trio"`,
		RawJSON:    []byte(`{"modes":[{"key":"trio"}]}`),
		SHA256Hash: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}
	cleaner.TrackGame(result.Game.ID)

	modes, err := env.Store.ListGameModesByGameID(ctx, result.Game.ID)
	if err != nil {
		t.Fatalf("ListGameModesByGameID: %v", err)
	}
	return result.Game.ID.String(), modes[0].ID.String()
}

type groupTableView struct {
	ID        string
	SeatSlots []struct {
		SeatKey string
		User    *struct{ ID string } `json:"user"`
	} `json:"seatSlots"`
	LookForGroupOptions []struct {
		QueueID string `json:"queueId"`
		Visible bool
		Enabled bool
	} `json:"lookForGroupOptions"`
	BackfillActive bool `json:"backfillActive"`
}

const groupTableFields = `
	id
	seatSlots { seatKey user { id } }
	lookForGroupOptions { queueId visible enabled }
	backfillActive
`

// The whole GraphQL path a group screen uses: the king asks the lobby for the rest of the
// match, a seated friend cannot, and the king withdraws it again. The store enforces the
// gate; this checks the resolver does not quietly widen it.
func TestStartAndCancelTableBackfillIsTheKingsAlone(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")

	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	gameID, modeID := registerThreeSeatGame(t, env, cleaner)

	_, kingCookie := createTestUserSession(t, ctx, env, cleaner)
	friend := createTestUser(t, ctx, env, cleaner, "backfill-friend-"+uuid.NewString()+"@example.com", "Friend")
	_, friendCookie := createTestUserSessionForUser(t, env, friend.ID)

	var created struct {
		CreatePrivateTable groupTableView `json:"createPrivateTable"`
	}
	if err := env.Client.Post(`mutation Create($gameId: ID!, $modeId: ID!) {
		createPrivateTable(gameId: $gameId, modeId: $modeId) { `+groupTableFields+` }
	}`, &created, client.AddCookie(kingCookie),
		client.Var("gameId", gameID), client.Var("modeId", modeID),
	); err != nil {
		t.Fatalf("createPrivateTable: %v", err)
	}
	table := created.CreatePrivateTable
	if len(table.SeatSlots) != 3 {
		t.Fatalf("expected 3 seats, got %d", len(table.SeatSlots))
	}

	var room struct {
		MyRoom struct{ InviteCode string } `json:"myRoom"`
	}
	if err := env.Client.Post(`query { myRoom { inviteCode } }`, &room,
		client.AddCookie(kingCookie)); err != nil {
		t.Fatalf("myRoom: %v", err)
	}

	sit := `mutation Sit($tableId: ID!, $seatKey: String!) { sitAtTable(tableId: $tableId, seatKey: $seatKey) { id } }`
	var sat struct {
		SitAtTable struct{ ID string } `json:"sitAtTable"`
	}
	// Taking the first seat is the whole of what makes them the king.
	if err := env.Client.Post(sit, &sat, client.AddCookie(kingCookie),
		client.Var("tableId", table.ID), client.Var("seatKey", table.SeatSlots[0].SeatKey),
	); err != nil {
		t.Fatalf("king sits: %v", err)
	}

	var joined struct {
		JoinRoom struct{ ID string } `json:"joinRoom"`
	}
	if err := env.Client.Post(`mutation Join($code: String!) { joinRoom(inviteCode: $code) { id } }`,
		&joined, client.AddCookie(friendCookie), client.Var("code", room.MyRoom.InviteCode),
	); err != nil {
		t.Fatalf("friend joins the room: %v", err)
	}
	if err := env.Client.Post(sit, &sat, client.AddCookie(friendCookie),
		client.Var("tableId", table.ID), client.Var("seatKey", table.SeatSlots[1].SeatKey),
	); err != nil {
		t.Fatalf("friend sits: %v", err)
	}

	readTable := func() groupTableView {
		t.Helper()
		var resp struct {
			MyRoom struct {
				Tables []groupTableView
			} `json:"myRoom"`
		}
		if err := env.Client.Post(`query { myRoom { tables { `+groupTableFields+` } } }`, &resp,
			client.AddCookie(friendCookie)); err != nil {
			t.Fatalf("read table: %v", err)
		}
		for _, tbl := range resp.MyRoom.Tables {
			if tbl.ID == table.ID {
				return tbl
			}
		}
		t.Fatalf("table %s not in the room", table.ID)
		return groupTableView{}
	}

	before := readTable()
	queueID := ""
	for _, opt := range before.LookForGroupOptions {
		if opt.Visible && opt.Enabled {
			queueID = opt.QueueID
			break
		}
	}
	if queueID == "" {
		t.Fatal("the partly-filled table offers no queue to ask")
	}

	backfill := `mutation Backfill($tableId: ID!, $queueId: ID!) {
		startTableBackfill(tableId: $tableId, queueId: $queueId) { queued queuedCount }
	}`
	var started struct {
		StartTableBackfill struct {
			Queued      bool
			QueuedCount *int `json:"queuedCount"`
		} `json:"startTableBackfill"`
	}
	// Seated, and still not theirs: filling the table ends the wait for anyone still on
	// their way, which is the king's call however it is spelled.
	if err := env.Client.Post(backfill, &started, client.AddCookie(friendCookie),
		client.Var("tableId", table.ID), client.Var("queueId", queueID),
	); err == nil {
		t.Fatal("expected a seated player who is not the king to be refused")
	}

	if err := env.Client.Post(backfill, &started, client.AddCookie(kingCookie),
		client.Var("tableId", table.ID), client.Var("queueId", queueID),
	); err != nil {
		t.Fatalf("startTableBackfill as the king: %v", err)
	}
	if !started.StartTableBackfill.Queued {
		t.Fatal("expected the request to report the group as queued")
	}
	// Live from the moment the group is queued, before any worker has built a forming
	// map — otherwise the button the friend just pressed would come straight back.
	during := readTable()
	if !during.BackfillActive {
		t.Fatal("expected the table to read as filling as soon as the request was made")
	}
	for _, opt := range during.LookForGroupOptions {
		if opt.Enabled {
			t.Fatal("expected no queue to be offered while a request is already live")
		}
	}

	// Still true once the worker has run and the group is on a forming map.
	reconcileQueue(t, env, queueID)
	if after := readTable(); !after.BackfillActive {
		t.Fatal("expected the table to read as filling once it is on the forming map")
	}

	// Withdrawing is the king's too — it puts the whole group back to waiting.
	cancel := `mutation Cancel($tableId: ID!) { cancelTableBackfill(tableId: $tableId) }`
	var cancelled struct {
		CancelTableBackfill bool `json:"cancelTableBackfill"`
	}
	if err := env.Client.Post(cancel, &cancelled, client.AddCookie(friendCookie),
		client.Var("tableId", table.ID),
	); err == nil {
		t.Fatal("expected a seated player who is not the king to be refused the cancel")
	}
	if err := env.Client.Post(cancel, &cancelled, client.AddCookie(kingCookie),
		client.Var("tableId", table.ID),
	); err != nil {
		t.Fatalf("cancelTableBackfill: %v", err)
	}
	if !cancelled.CancelTableBackfill {
		t.Fatal("expected the cancel to report that it withdrew a live request")
	}
	if after := readTable(); after.BackfillActive {
		t.Fatal("expected the table to stop reading as filling after the cancel")
	}
}
