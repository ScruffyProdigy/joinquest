package graph

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/scruffyprodigy/playhub/internal/store"
)

func TestMyRoomTableSeatsIncludeUser(t *testing.T) {
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")

	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	_, hostCookie := createTestUserSession(t, ctx, env, cleaner)

	var gameResp struct {
		Game struct {
			ID    string
			Modes []struct {
				ID string
			} `json:"modes"`
		} `json:"game"`
	}
	if err := env.Client.Post(`query Game($id: ID!) {
		game(id: $id) { id modes { id } }
	}`, &gameResp, client.AddCookie(hostCookie), client.Var("id", store.DemoPrimaryGameIDStr)); err != nil {
		t.Fatalf("game query: %v", err)
	}
	if len(gameResp.Game.Modes) == 0 {
		t.Fatal("expected demo game with at least one mode")
	}
	gameID := gameResp.Game.ID
	modeID := gameResp.Game.Modes[0].ID

	var tableResp struct {
		CreatePrivateTable struct {
			ID        string
			SeatSlots []struct {
				SeatKey string
			} `json:"seatSlots"`
		} `json:"createPrivateTable"`
	}
	createTableQuery := `mutation CreateTable($gameId: ID!, $modeId: ID!) {
		createPrivateTable(gameId: $gameId, modeId: $modeId) {
			id
			seatSlots { seatKey }
		}
	}`
	if err := env.Client.Post(createTableQuery, &tableResp, client.AddCookie(hostCookie),
		client.Var("gameId", gameID),
		client.Var("modeId", modeID),
	); err != nil {
		t.Fatalf("createPrivateTable: %v", err)
	}
	if len(tableResp.CreatePrivateTable.SeatSlots) == 0 {
		t.Fatal("expected seat slots on new table")
	}
	seatKey := tableResp.CreatePrivateTable.SeatSlots[0].SeatKey

	var sitResp struct {
		SitAtTable struct {
			ID string
		} `json:"sitAtTable"`
	}
	sitQuery := `mutation Sit($tableId: ID!, $seatKey: String!) {
		sitAtTable(tableId: $tableId, seatKey: $seatKey) { id }
	}`
	if err := env.Client.Post(sitQuery, &sitResp, client.AddCookie(hostCookie),
		client.Var("tableId", tableResp.CreatePrivateTable.ID),
		client.Var("seatKey", seatKey),
	); err != nil {
		t.Fatalf("sitAtTable: %v", err)
	}

	var roomResp struct {
		MyRoom struct {
			Tables []struct {
				Seats []struct {
					SeatKey string
					User    *struct {
						DisplayName string
					} `json:"user"`
				} `json:"seats"`
			} `json:"tables"`
		} `json:"myRoom"`
	}
	myRoomQuery := `query {
		myRoom {
			tables {
				seats {
					seatKey
					user { displayName }
				}
			}
		}
	}`
	if err := env.Client.Post(myRoomQuery, &roomResp, client.AddCookie(hostCookie)); err != nil {
		t.Fatalf("myRoom with table seats: %v", err)
	}
	if len(roomResp.MyRoom.Tables) == 0 {
		t.Fatal("expected table on myRoom")
	}
	seats := roomResp.MyRoom.Tables[0].Seats
	if len(seats) != 1 {
		t.Fatalf("expected 1 seated player, got %d", len(seats))
	}
	if seats[0].SeatKey != seatKey {
		t.Fatalf("seatKey = %q, want %q", seats[0].SeatKey, seatKey)
	}
	if seats[0].User == nil || seats[0].User.DisplayName == "" {
		t.Fatal("expected seated user displayName on myRoom.tables.seats.user")
	}
}
