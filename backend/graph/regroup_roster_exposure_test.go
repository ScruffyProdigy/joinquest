package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// requireSelectionRejected asserts that a field selection is not part of the schema at all.
// It cannot use postGraphQL: a rejected selection comes back as HTTP 422, which that helper
// treats as a broken test rather than the answer under test.
func requireSelectionRejected(t *testing.T, env *queueIntegrationEnv, query string, cookie *http.Cookie) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"query": query})
	if err != nil {
		t.Fatalf("marshal probe: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	env.Handler.ServeHTTP(rec, req)

	var resp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode probe response: %v body=%s", err, rec.Body.String())
	}
	if len(resp.Errors) == 0 {
		t.Errorf("the schema still answers %s; HTTP %d body=%s", query, rec.Code, rec.Body.String())
	}
}

// nonParticipantRosterQuery is what a backfill joiner's client actually sends: identity,
// seat and intent. Selecting it as a stranger is legitimate — the pending-seat display is
// the reason Table.regroupRoster has no participation gate in the first place.
const nonParticipantRosterQuery = `query {
	myRoom {
		tables {
			id
			regroupRoster {
				user { id displayName }
				role
				regroup
			}
		}
	}
}`

// seedRegroupTableWithStranger plays a match between A and B, has A claim the regroup
// table, and walks a third player — who never played that match — into the table's room
// the way the king's Look for group backfill does. It returns the match, the stranger's
// session cookie and the regroup table id.
func seedRegroupTableWithStranger(t *testing.T, env *queueIntegrationEnv, cleaner *store.TestCleaner) (finishedMatch, *http.Cookie, string) {
	t.Helper()
	ctx := context.Background()

	match := seedFinishedPartyMatch(t, env, cleaner)
	claim := playAgain(t, env, match.sessionID, match.cookieA)
	if len(claim.Errors) > 0 || claim.Data.PlayAgain == nil {
		t.Fatalf("playAgain A: %+v", claim.Errors)
	}

	stranger := createTestUser(t, ctx, env, cleaner,
		"regroup-stranger-"+uuid.NewString()+"@example.com", "Stranger")
	_, cookie := createTestUserSessionForUser(t, env, stranger.ID)

	joinMutation := `mutation Join($inviteCode: String!) { joinRoom(inviteCode: $inviteCode) { id } }`
	requireNoGraphQLErrors(t, postGraphQL(t, env.Handler, joinMutation,
		map[string]any{"inviteCode": claim.Data.PlayAgain.InviteCode}, cookie))

	return match, cookie, claim.Data.PlayAgain.Table.ID
}

// TestTableRegroupRosterHidesStandingsFromNonParticipants is the JQ-174 regression, and the
// non-participant case the earlier roster tests never covered — they all query as A or B,
// who played.
//
// A regroup table is joinable by people who did not play the originating match, so
// regroupRoster deliberately carries no requireMatchParticipant gate. The protection has to
// live in the payload instead: RegroupRosterEntry has no placement, winner or finish reason
// to read at all, so a stranger seated by backfill cannot learn how a match they never
// played ended.
func TestTableRegroupRosterHidesStandingsFromNonParticipants(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)

	match, strangerCookie, _ := seedRegroupTableWithStranger(t, env, cleaner)

	// The standings are not blanked for this caller — they are not fields of this type.
	// A participant asking the same question is refused too: the type is the boundary,
	// not a per-caller decision a later refactor could quietly drop.
	for _, field := range []string{"placement", "winner", "reason", "finished", "finishedAt"} {
		requireSelectionRejected(t, env,
			`query { myRoom { tables { regroupRoster { `+field+` } } } }`, strangerCookie)
	}

	// matchResult is the gated route to the same information, and it stays shut.
	denied := queryMatchResult(t, env, match.sessionID, strangerCookie)
	if len(denied.Errors) == 0 {
		t.Error("matchResult answered a non-participant")
	}
	if denied.Data.MatchResult != nil {
		t.Error("matchResult returned a payload to a non-participant")
	}
}

// TestTableRegroupRosterStillNamesPendingSeatsForNonParticipants is the other half of the
// fix: narrowing the payload must not cost the feature the field exists for. A backfill
// joiner still has to see that the open seat beside them belongs to a named player who has
// not answered yet, rather than to nobody.
func TestTableRegroupRosterStillNamesPendingSeatsForNonParticipants(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)

	match, strangerCookie, tableID := seedRegroupTableWithStranger(t, env, cleaner)

	body := postGraphQL(t, env.Handler, nonParticipantRosterQuery, nil, strangerCookie)
	requireNoGraphQLErrors(t, body)
	var resp struct {
		Data struct {
			MyRoom *struct {
				Tables []struct {
					ID            string `json:"id"`
					RegroupRoster []struct {
						User struct {
							ID          string `json:"id"`
							DisplayName string `json:"displayName"`
						} `json:"user"`
						Role    *string `json:"role"`
						Regroup string  `json:"regroup"`
					} `json:"regroupRoster"`
				} `json:"tables"`
			} `json:"myRoom"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode roster: %v body=%s", err, body)
	}
	if resp.Data.MyRoom == nil {
		t.Fatalf("myRoom is null for the backfill joiner; body=%s", body)
	}

	found := false
	for _, tbl := range resp.Data.MyRoom.Tables {
		if tbl.ID != tableID {
			continue
		}
		found = true
		if len(tbl.RegroupRoster) != 2 {
			t.Fatalf("roster = %d entries, want 2; body=%s", len(tbl.RegroupRoster), body)
		}
		states := map[string]string{}
		for _, entry := range tbl.RegroupRoster {
			states[entry.User.ID] = entry.Regroup
			if entry.User.DisplayName == "" {
				t.Errorf("roster entry %s has no displayName — the pending seat cannot be named", entry.User.ID)
			}
			// role is what attachPendingRegroup pairs on (seatKey === role). Without it the
			// ghosted seat lands on the wrong slot, or on none.
			if entry.Role == nil || *entry.Role == "" {
				t.Errorf("roster entry %s has no role to pair a pending seat on", entry.User.ID)
			}
		}
		if states[match.userA.ID.String()] != "IN" {
			t.Errorf("A = %q, want IN", states[match.userA.ID.String()])
		}
		if states[match.userB.ID.String()] != "PENDING" {
			t.Errorf("B = %q, want PENDING", states[match.userB.ID.String()])
		}
	}
	if !found {
		t.Fatalf("regroup table %s not visible to the backfill joiner; body=%s", tableID, body)
	}
}

// TestMatchResultStillCarriesStandingsForParticipants pins the other side of the split.
// Moving the table's roster onto a narrower type must not have narrowed the gated route the
// results screen depends on.
func TestMatchResultStillCarriesStandingsForParticipants(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)

	match := seedFinishedMatch(t, env, cleaner)
	resp := queryMatchResult(t, env, match.sessionID, match.cookieA)
	if len(resp.Errors) > 0 {
		t.Fatalf("matchResult errors for a participant: %+v", resp.Errors)
	}
	if resp.Data.MatchResult == nil {
		t.Fatal("matchResult is null for a participant")
	}

	winners, placed := 0, 0
	for _, p := range resp.Data.MatchResult.Participants {
		if p.Winner {
			winners++
		}
		if p.Placement != nil {
			placed++
		}
	}
	if winners != 1 {
		t.Errorf("winners = %d, want 1 — participants lost their standings", winners)
	}
	if placed == 0 {
		t.Error("no participant carries a placement — participants lost their standings")
	}
}

// TestTableSurfaceCarriesNoEmail covers the seats rendered beside the roster on the same
// card. JQ-135 moved the match roster to PublicPlayer to keep emails away from strangers
// matched by the catalog queue; the seats and the king were left as User, which made them
// the looser surface on the very same table. They are PublicPlayer now, and asking for an
// email is a schema error rather than a value.
func TestTableSurfaceCarriesNoEmail(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)

	_, strangerCookie, _ := seedRegroupTableWithStranger(t, env, cleaner)

	for _, selection := range []string{
		"seats { user { email } }",
		"seatSlots { user { email } }",
		"king { email }",
		"regroupRoster { user { email } }",
	} {
		requireSelectionRejected(t, env,
			`query { myRoom { tables { `+selection+` } } }`, strangerCookie)
	}
}
