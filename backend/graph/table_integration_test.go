package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/google/uuid"
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

// TestTableRegroupRosterReturnsEmptySliceNotNil is the code-level half of the non-null list
// contract. TestTableRegroupRosterEmptyForOrdinaryTable above cannot see it: gqlgen marshals
// a nil and an empty Go slice identically ("[]") for a non-null list, so only a direct call
// on the resolver catches a refactor to `var roster []*model.MatchParticipantResult;
// return roster, nil`.
func TestTableRegroupRosterReturnsEmptySliceNotNil(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	resolver := &Resolver{Store: env.Store}

	// A table id nothing points at exercises the "no originating match" branch. The
	// resolver only uses the id for the reverse lookup, so no table row is needed.
	roster, err := resolver.Table().RegroupRoster(context.Background(), &model.Table{ID: uuid.NewString()})
	if err != nil {
		t.Fatalf("RegroupRoster: %v", err)
	}
	if roster == nil {
		t.Fatal("RegroupRoster returned a nil slice; [MatchParticipantResult!]! needs an empty one")
	}
	if len(roster) != 0 {
		t.Fatalf("roster = %d entries, want 0 for a table with no originating match", len(roster))
	}
}

// roomTablesQuery reads everything a regroup-table test needs about a room's tables in one
// round trip: the seat layout (to find an open seat) and the roster under test.
const roomTablesQuery = `query {
	myRoom {
		tables {
			id
			seatSlots { seatKey }
			seats { seatKey user { id } }
			regroupRoster {
				user { id }
				regroup
			}
		}
	}
}`

type roomTablesResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Data struct {
		MyRoom *struct {
			Tables []struct {
				ID        string `json:"id"`
				SeatSlots []struct {
					SeatKey string `json:"seatKey"`
				} `json:"seatSlots"`
				Seats []struct {
					SeatKey string `json:"seatKey"`
					User    struct {
						ID string `json:"id"`
					} `json:"user"`
				} `json:"seats"`
				RegroupRoster []struct {
					User struct {
						ID string `json:"id"`
					} `json:"user"`
					Regroup string `json:"regroup"`
				} `json:"regroupRoster"`
			} `json:"tables"`
		} `json:"myRoom"`
	} `json:"data"`
}

// readRoomTable fetches the caller's room tables and fails unless tableID is among them.
func readRoomTable(t *testing.T, env *queueIntegrationEnv, cookie *http.Cookie, tableID string) roomTablesResponse {
	t.Helper()
	body := postGraphQL(t, env.Handler, roomTablesQuery, nil, cookie)
	var resp roomTablesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode myRoom: %v body=%s", err, body)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("myRoom errors: %+v body=%s", resp.Errors, body)
	}
	if resp.Data.MyRoom == nil {
		t.Fatalf("myRoom is null; body=%s", body)
	}
	for _, tbl := range resp.Data.MyRoom.Tables {
		if tbl.ID == tableID {
			return resp
		}
	}
	t.Fatalf("table %s not on myRoom; body=%s", tableID, body)
	return resp
}

// TestTableRegroupRosterFollowsTheLatestMatch is the second-round-of-play regression.
// game_sessions.regroup_table_id has no uniqueness constraint and is never cleared, so a
// room that plays twice at the same table leaves TWO rows carrying that table id: playAgain
// stamps the first match's session when it claims the table, and CompleteSession's table
// reset stamps the second match's session when that match ends. An unordered reverse lookup
// may answer with either, and for an unindexed column Postgres tends to hand back the
// oldest — naming a player who has since left and omitting the one who took their seat.
//
// The scenario: A and B play match 1, B leaves and C backfills, A and C play match 2 at the
// same table. The roster must then name A and C, not A and B.
func TestTableRegroupRosterFollowsTheLatestMatch(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	// Match 1, from the queue: A and B. Both accept play-again, converging on regroup
	// table T and stamping session 1's regroup_table_id.
	first := seedFinishedMatch(t, env, cleaner)
	claimA := playAgain(t, env, first.sessionID, first.cookieA)
	if len(claimA.Errors) > 0 || claimA.Data.PlayAgain == nil {
		t.Fatalf("playAgain A: %+v", claimA.Errors)
	}
	claimB := playAgain(t, env, first.sessionID, first.cookieB)
	if len(claimB.Errors) > 0 || claimB.Data.PlayAgain == nil {
		t.Fatalf("playAgain B: %+v", claimB.Errors)
	}
	tableID := claimA.Data.PlayAgain.Table.ID
	if claimB.Data.PlayAgain.Table.ID != tableID {
		t.Fatalf("players landed on different tables: %s vs %s", tableID, claimB.Data.PlayAgain.Table.ID)
	}
	inviteCode := claimA.Data.PlayAgain.InviteCode

	// B walks away from the regroup table; C backfills into the seat B vacated.
	leaveMutation := `mutation Leave($tableId: ID!) { leaveTable(tableId: $tableId) }`
	requireNoGraphQLErrors(t, postGraphQL(t, env.Handler, leaveMutation,
		map[string]any{"tableId": tableID}, first.cookieB))

	userC, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "regroup-backfill-c-" + uuid.NewString() + "@example.com",
		DisplayName: "Backfill C",
	})
	if err != nil {
		t.Fatalf("CreateUser C: %v", err)
	}
	cleaner.TrackUser(userC.ID)
	_, cookieC := createTestUserSessionForUser(t, env, userC.ID)

	joinMutation := `mutation Join($inviteCode: String!) { joinRoom(inviteCode: $inviteCode) { id } }`
	requireNoGraphQLErrors(t, postGraphQL(t, env.Handler, joinMutation,
		map[string]any{"inviteCode": inviteCode}, cookieC))

	openSeat := ""
	before := readRoomTable(t, env, first.cookieA, tableID)
	for _, tbl := range before.Data.MyRoom.Tables {
		if tbl.ID != tableID {
			continue
		}
		taken := map[string]bool{}
		for _, seat := range tbl.Seats {
			taken[seat.SeatKey] = true
			if seat.User.ID == first.userB.ID.String() {
				t.Fatalf("player B is still seated at %s after leaveTable", tableID)
			}
		}
		for _, slot := range tbl.SeatSlots {
			if !taken[slot.SeatKey] {
				openSeat = slot.SeatKey
				break
			}
		}
	}
	if openSeat == "" {
		t.Fatal("no open seat at the regroup table for C to backfill into")
	}

	sitMutation := `mutation Sit($tableId: ID!, $seatKey: String!) {
		sitAtTable(tableId: $tableId, seatKey: $seatKey) { id }
	}`
	requireNoGraphQLErrors(t, postGraphQL(t, env.Handler, sitMutation,
		map[string]any{"tableId": tableID, "seatKey": openSeat}, cookieC))

	// Match 2: A (the king) starts the table, and the game reports it complete. That
	// completion runs resetRoomTableAfterSessionTx, which stamps session 2 with the SAME
	// regroup_table_id session 1 already carries.
	startMutation := `mutation Start($tableId: ID!) { startTable(tableId: $tableId) { sessionId } }`
	startBody := postGraphQL(t, env.Handler, startMutation, map[string]any{"tableId": tableID}, first.cookieA)
	requireNoGraphQLErrors(t, startBody)
	var startResp struct {
		Data struct {
			StartTable struct {
				SessionID *string `json:"sessionId"`
			} `json:"startTable"`
		} `json:"data"`
	}
	if err := json.Unmarshal(startBody, &startResp); err != nil {
		t.Fatalf("decode startTable: %v body=%s", err, startBody)
	}
	if startResp.Data.StartTable.SessionID == nil {
		t.Fatalf("startTable returned no sessionId; body=%s", startBody)
	}
	secondSessionID, err := uuid.Parse(*startResp.Data.StartTable.SessionID)
	if err != nil {
		t.Fatalf("parse second session id: %v", err)
	}
	if secondSessionID == first.sessionID {
		t.Fatal("startTable reused the first match's session id")
	}
	reportMatchResult(t, env, secondSessionID, "COMPLETED")

	// Precondition for the whole test: table T really is stamped on both sessions now, so
	// the lookup genuinely has to choose. If this ever drops to one row the assertions
	// below would pass for the wrong reason.
	var stamped int
	if err := env.DB.QueryRow(
		`SELECT COUNT(*) FROM game_sessions WHERE regroup_table_id = $1`, tableID,
	).Scan(&stamped); err != nil {
		t.Fatalf("count sessions stamped with the table: %v", err)
	}
	if stamped != 2 {
		t.Fatalf("%d sessions carry regroup_table_id = %s, want 2 (both matches)", stamped, tableID)
	}

	// Both sessions now point at table T. The roster must describe match 2.
	after := readRoomTable(t, env, first.cookieA, tableID)
	var roster map[string]string
	for _, tbl := range after.Data.MyRoom.Tables {
		if tbl.ID != tableID {
			continue
		}
		roster = map[string]string{}
		for _, p := range tbl.RegroupRoster {
			roster[p.User.ID] = p.Regroup
		}
	}
	if len(roster) != 2 {
		t.Fatalf("roster = %d entries (%v), want 2 for match 2", len(roster), roster)
	}
	if _, ok := roster[first.userA.ID.String()]; !ok {
		t.Errorf("player A missing from the roster: %v", roster)
	}
	if _, ok := roster[userC.ID.String()]; !ok {
		t.Errorf("backfill player C missing from the roster: %v", roster)
	}
	if _, ok := roster[first.userB.ID.String()]; ok {
		t.Errorf("roster names player B, who left before match 2 — the lookup returned the stale session: %v", roster)
	}
}
