package graph

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/scruffyprodigy/playhub/graph/model"
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

// playAgainWithRosterMutation extends the ordinary playAgain mutation to also select the
// resulting table's regroupRoster, so a test can inspect both in one round trip.
const playAgainWithRosterMutation = `mutation PlayAgain($matchId: ID!) {
	playAgain(matchId: $matchId) {
		table {
			id
			regroupRoster {
				user { id }
				regroup
			}
		}
	}
}`

type playAgainWithRosterResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Data struct {
		PlayAgain *struct {
			Table struct {
				ID            string `json:"id"`
				RegroupRoster []struct {
					User struct {
						ID string `json:"id"`
					} `json:"user"`
					Regroup string `json:"regroup"`
				} `json:"regroupRoster"`
			} `json:"table"`
		} `json:"playAgain"`
	} `json:"data"`
}

// TestTableRegroupRosterNamesPendingPlayers covers the whole point of the field: the
// regroup table names the match's originating roster, including a player who has not yet
// answered play-again. Without this, an open seat at the regroup table is indistinguishable
// from an ordinary backfill-open seat.
func TestTableRegroupRosterNamesPendingPlayers(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	body := postGraphQL(t, env.Handler, playAgainWithRosterMutation,
		map[string]any{"matchId": match.sessionID.String()}, match.cookieA)
	var resp playAgainWithRosterResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode playAgain: %v body=%s", err, body)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("playAgain errors: %+v", resp.Errors)
	}
	if resp.Data.PlayAgain == nil {
		t.Fatal("playAgain returned a null result")
	}

	roster := resp.Data.PlayAgain.Table.RegroupRoster
	if len(roster) != 2 {
		t.Fatalf("roster = %d, want 2", len(roster))
	}
	states := map[string]model.RegroupState{}
	for _, p := range roster {
		states[p.User.ID] = model.RegroupState(p.Regroup)
	}
	if states[match.userA.ID.String()] != model.RegroupStateIn {
		t.Errorf("userA = %v, want IN", states[match.userA.ID.String()])
	}
	if states[match.userB.ID.String()] != model.RegroupStatePending {
		t.Errorf("userB = %v, want PENDING", states[match.userB.ID.String()])
	}
}

// TestTableRegroupRosterEmptyForOrdinaryTable pins the non-null-list contract: a table with
// no originating match must return an explicitly empty roster, never nil (which would
// marshal identically but signal a mapping bug rather than "no such match").
func TestTableRegroupRosterEmptyForOrdinaryTable(t *testing.T) {
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

	var tableResp struct {
		CreatePrivateTable struct {
			ID            string
			RegroupRoster []struct {
				User struct {
					ID string
				} `json:"user"`
			} `json:"regroupRoster"`
		} `json:"createPrivateTable"`
	}
	createTableQuery := `mutation CreateTable($gameId: ID!, $modeId: ID!) {
		createPrivateTable(gameId: $gameId, modeId: $modeId) {
			id
			regroupRoster {
				user { id }
			}
		}
	}`
	if err := env.Client.Post(createTableQuery, &tableResp, client.AddCookie(hostCookie),
		client.Var("gameId", gameResp.Game.ID),
		client.Var("modeId", gameResp.Game.Modes[0].ID),
	); err != nil {
		t.Fatalf("createPrivateTable: %v", err)
	}
	// The wire encoding of a nil vs. an empty Go slice is identical ("[]"), so this only
	// pins the observable contract: no roster entries, and (implicitly, since the query
	// above returned without error) no GraphQL "must not be null" error from a nil slice
	// landing on the schema's non-null list.
	if len(tableResp.CreatePrivateTable.RegroupRoster) != 0 {
		t.Fatalf("regroupRoster = %d entries, want 0 for an ordinary table",
			len(tableResp.CreatePrivateTable.RegroupRoster))
	}
}
