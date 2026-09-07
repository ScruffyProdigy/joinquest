package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/store"
)

// matchResultQuery selects every field the results screen reads, so a mapping slip in any
// of them surfaces here rather than in the frontend task that consumes them.
const matchResultQuery = `query Result($matchId: ID!) {
	matchResult(matchId: $matchId) {
		matchId
		game { id }
		status
		reported
		complete
		endedAt
		regroupInviteCode
		participants {
			user { id displayName }
			role
			finished
			finishedAt
			reason
			placement
			winner
			regroup
		}
	}
}`

type matchResultResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Data struct {
		MatchResult *struct {
			MatchID string `json:"matchId"`
			Game    struct {
				ID string `json:"id"`
			} `json:"game"`
			Status            *string `json:"status"`
			Reported          bool    `json:"reported"`
			Complete          bool    `json:"complete"`
			EndedAt           *string `json:"endedAt"`
			RegroupInviteCode *string `json:"regroupInviteCode"`
			Participants      []struct {
				User struct {
					ID          string  `json:"id"`
					DisplayName *string `json:"displayName"`
				} `json:"user"`
				Role       *string `json:"role"`
				Finished   bool    `json:"finished"`
				FinishedAt *string `json:"finishedAt"`
				Reason     *string `json:"reason"`
				Placement  *int    `json:"placement"`
				Winner     bool    `json:"winner"`
				Regroup    string  `json:"regroup"`
			} `json:"participants"`
		} `json:"matchResult"`
	} `json:"data"`
}

// finishedMatch is a completed two-player demo match: A finished first and won, B was
// never reported by the game at all.
type finishedMatch struct {
	sessionID uuid.UUID
	userA     *store.User
	userB     *store.User
	cookieA   *http.Cookie
	cookieB   *http.Cookie
}

func seedFinishedMatch(t *testing.T, env *queueIntegrationEnv, cleaner *store.TestCleaner) finishedMatch {
	t.Helper()
	ctx := context.Background()

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "match-result-pepper")

	clearDemoQueue(t, env.Store)
	provisioner := &syncProvisioner{}
	env.resolverWithProvisioner(t, provisioner)

	userA, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "match-result-a-" + uuid.NewString() + "@example.com",
		DisplayName: "Result A",
	})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	_, cookieA := createTestUserSessionForUser(t, env, userA.ID)

	userB, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email:       "match-result-b-" + uuid.NewString() + "@example.com",
		DisplayName: "Result B",
	})
	if err != nil {
		t.Fatalf("CreateUser B: %v", err)
	}
	cleaner.TrackUser(userB.ID)
	_, cookieB := createTestUserSessionForUser(t, env, userB.ID)

	joinQuery := `mutation Join($id: ID!) { joinQueue(queueId: $id) { queued queuedCount } }`
	vars := map[string]any{"id": demoDefaultQueueID}
	postGraphQL(t, env.Handler, joinQuery, vars, cookieA)
	postGraphQL(t, env.Handler, joinQuery, vars, cookieB)
	flushFormingWorker(t, env, ctx, uuid.MustParse(demoDefaultQueueID))
	waitForProvisionCalls(t, provisioner, 1)

	matchID := provisioner.lastCall().Assignment.ExternalMatchID
	sessionID, err := uuid.Parse(matchID)
	if err != nil {
		t.Fatalf("parse session id: %v", err)
	}

	serviceToken, err := auth.FormatGameServiceToken(uuid.MustParse(store.DemoPrimaryGameIDStr))
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}

	finishMutation := `mutation Finish($matchId: ID!, $lobbyUserId: ID!, $reason: PlayerFinishReason!, $placement: Int) {
		reportPlayerFinished(matchId: $matchId, lobbyUserId: $lobbyUserId, reason: $reason, placement: $placement)
	}`
	requireNoGraphQLErrors(t, postGraphQLWithBearer(t, env.Handler, serviceToken, finishMutation, map[string]any{
		"matchId":     matchID,
		"lobbyUserId": userA.ID.String(),
		"reason":      "COMPLETED",
		"placement":   1,
	}))

	resultMutation := `mutation Report($matchId: ID!, $status: MatchResultStatus!, $winnerLobbyUserIds: [ID!]) {
		reportMatchResult(matchId: $matchId, status: $status, winnerLobbyUserIds: $winnerLobbyUserIds)
	}`
	requireNoGraphQLErrors(t, postGraphQLWithBearer(t, env.Handler, serviceToken, resultMutation, map[string]any{
		"matchId":            matchID,
		"status":             "COMPLETED",
		"winnerLobbyUserIds": []string{userA.ID.String()},
	}))

	return finishedMatch{sessionID: sessionID, userA: userA, userB: userB, cookieA: cookieA, cookieB: cookieB}
}

func requireNoGraphQLErrors(t *testing.T, body []byte) {
	t.Helper()
	var resp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("graphql errors: %+v body=%s", resp.Errors, body)
	}
}

func queryMatchResult(t *testing.T, env *queueIntegrationEnv, sessionID uuid.UUID, cookie *http.Cookie) matchResultResponse {
	t.Helper()
	cookies := []*http.Cookie{cookie}
	if cookie == nil {
		cookies = nil // signed out
	}
	body := postGraphQL(t, env.Handler, matchResultQuery, map[string]any{"matchId": sessionID.String()}, cookies...)
	var resp matchResultResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode matchResult: %v body=%s", err, body)
	}
	return resp
}

// TestMatchResultVisibleOnlyToParticipants is the authorization gate: a stored result
// belongs to the people who played the match and to nobody else. A signed-in stranger who
// happens to know the match id must get an error, not the roster.
func TestMatchResultVisibleOnlyToParticipants(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	// A participant sees it. Without this half the test could pass on a query that is
	// simply broken for everyone.
	asParticipant := queryMatchResult(t, env, match.sessionID, match.cookieA)
	if len(asParticipant.Errors) > 0 {
		t.Fatalf("participant was refused: %+v", asParticipant.Errors)
	}
	if asParticipant.Data.MatchResult == nil {
		t.Fatal("participant got a null matchResult")
	}
	if len(asParticipant.Data.MatchResult.Participants) != 2 {
		t.Fatalf("participants = %d, want 2", len(asParticipant.Data.MatchResult.Participants))
	}

	// A stranger does not, and gets nothing back to compensate.
	_, outsiderCookie := createTestUserSession(t, context.Background(), env, cleaner)
	asOutsider := queryMatchResult(t, env, match.sessionID, outsiderCookie)
	if len(asOutsider.Errors) == 0 {
		t.Fatal("expected matchResult to refuse a non-participant")
	}
	if asOutsider.Data.MatchResult != nil {
		t.Fatalf("non-participant received match data: %+v", asOutsider.Data.MatchResult)
	}

	// Signed out is refused too.
	anonymous := queryMatchResult(t, env, match.sessionID, nil)
	if len(anonymous.Errors) == 0 {
		t.Fatal("expected matchResult to refuse an anonymous caller")
	}
	if anonymous.Data.MatchResult != nil {
		t.Fatalf("anonymous caller received match data: %+v", anonymous.Data.MatchResult)
	}
}

// TestMatchResultReportsStoredOutcome covers the mapping. The load-bearing assertion is
// player B's null reason: the game never reported B, and that has to stay distinguishable
// from "the game said B is done".
func TestMatchResultReportsStoredOutcome(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	resp := queryMatchResult(t, env, match.sessionID, match.cookieA)
	if len(resp.Errors) > 0 {
		t.Fatalf("matchResult errors: %+v", resp.Errors)
	}
	got := resp.Data.MatchResult
	if got == nil {
		t.Fatal("matchResult is null")
	}
	if got.MatchID != match.sessionID.String() {
		t.Errorf("matchId = %q, want %q", got.MatchID, match.sessionID)
	}
	if got.Game.ID != store.DemoPrimaryGameIDStr {
		t.Errorf("game.id = %q, want %q", got.Game.ID, store.DemoPrimaryGameIDStr)
	}
	if got.Status == nil || *got.Status != "COMPLETED" {
		t.Errorf("status = %v, want COMPLETED", got.Status)
	}
	if !got.Reported {
		t.Error("reported = false, want true after reportMatchResult")
	}
	if !got.Complete {
		t.Error("complete = false, want true for a completed session")
	}
	if got.EndedAt == nil {
		t.Error("endedAt is null on a completed session")
	}
	if got.RegroupInviteCode != nil {
		t.Errorf("regroupInviteCode = %v, want null before any table is claimed", *got.RegroupInviteCode)
	}

	byUser := map[string]int{}
	for i, p := range got.Participants {
		byUser[p.User.ID] = i
	}
	aIdx, ok := byUser[match.userA.ID.String()]
	if !ok {
		t.Fatalf("player A missing from participants: %+v", got.Participants)
	}
	bIdx, ok := byUser[match.userB.ID.String()]
	if !ok {
		t.Fatalf("player B missing from participants: %+v", got.Participants)
	}

	a := got.Participants[aIdx]
	if a.User.DisplayName == nil || *a.User.DisplayName != "Result A" {
		t.Errorf("player A displayName = %v, want Result A", a.User.DisplayName)
	}
	if !a.Finished || a.FinishedAt == nil {
		t.Errorf("player A finished = %v / finishedAt = %v, want finished", a.Finished, a.FinishedAt)
	}
	if a.Reason == nil || *a.Reason != "COMPLETED" {
		t.Errorf("player A reason = %v, want COMPLETED", a.Reason)
	}
	if a.Placement == nil || *a.Placement != 1 {
		t.Errorf("player A placement = %v, want 1", a.Placement)
	}
	if !a.Winner {
		t.Error("player A winner = false, want true")
	}
	if a.Role == nil || *a.Role == "" {
		t.Errorf("player A role = %v, want a non-empty role", a.Role)
	}
	if a.Regroup != "PENDING" {
		t.Errorf("player A regroup = %q, want PENDING before anyone answers", a.Regroup)
	}

	b := got.Participants[bIdx]
	if b.Finished {
		t.Error("player B finished = true, but the game never reported B")
	}
	if b.Reason != nil {
		t.Errorf("player B reason = %v, want null — the game never reported B", *b.Reason)
	}
	if b.Winner {
		t.Error("player B winner = true, want false")
	}
	if b.Regroup != "PENDING" {
		t.Errorf("player B regroup = %q, want PENDING", b.Regroup)
	}
}

// TestMatchResultSurfacesClaimedRegroupTable covers the other half of regroupInviteCode:
// once a player claims the regroup table, every participant can route to it, and the
// claimant reads as IN while everyone else stays PENDING.
func TestMatchResultSurfacesClaimedRegroupTable(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	match := seedFinishedMatch(t, env, cleaner)

	_, room, err := env.Store.ClaimRegroupTable(ctx, match.sessionID, match.userA.ID)
	if err != nil {
		t.Fatalf("ClaimRegroupTable: %v", err)
	}

	// Read it back as B, who has not answered: the invite code is the match's, not the
	// caller's own.
	resp := queryMatchResult(t, env, match.sessionID, match.cookieB)
	if len(resp.Errors) > 0 {
		t.Fatalf("matchResult errors: %+v", resp.Errors)
	}
	got := resp.Data.MatchResult
	if got == nil {
		t.Fatal("matchResult is null")
	}
	if got.RegroupInviteCode == nil {
		t.Fatal("regroupInviteCode is null after a table was claimed")
	}
	if *got.RegroupInviteCode != room.InviteCode {
		t.Errorf("regroupInviteCode = %q, want %q", *got.RegroupInviteCode, room.InviteCode)
	}

	states := map[string]string{}
	for _, p := range got.Participants {
		states[p.User.ID] = p.Regroup
	}
	if states[match.userA.ID.String()] != "IN" {
		t.Errorf("claimant regroup = %q, want IN", states[match.userA.ID.String()])
	}
	if states[match.userB.ID.String()] != "PENDING" {
		t.Errorf("non-answering player regroup = %q, want PENDING", states[match.userB.ID.String()])
	}
}

// postGraphQLTolerantOfStatus is postGraphQL without its "HTTP >= 400 fails the test" rule:
// gqlgen answers a query that fails *validation* with 422, and a rejected selection is
// exactly what the email test below asserts.
func postGraphQLTolerantOfStatus(t *testing.T, handler http.Handler, query string, variables map[string]any, cookies ...*http.Cookie) (int, []byte) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// TestMatchResultRosterCannotExposeEmail pins the privacy decision the schema encodes: a
// queue match introduces strangers, so the roster is PublicPlayer, which has no email
// field at all. Asking for one must be rejected outright — if MatchParticipantResult.user
// ever regresses to User, this query starts succeeding and hands one stranger another's
// real address.
func TestMatchResultRosterCannotExposeEmail(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	const emailQuery = `query Leak($matchId: ID!) {
		matchResult(matchId: $matchId) {
			participants { user { email } }
		}
	}`
	// Asked by a genuine participant, so the only thing that can refuse it is the type.
	status, body := postGraphQLTolerantOfStatus(t, env.Handler, emailQuery,
		map[string]any{"matchId": match.sessionID.String()}, match.cookieA)

	var resp matchResultResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	if len(resp.Errors) == 0 {
		t.Fatalf("selecting participants.user.email was accepted (HTTP %d): %s", status, body)
	}
	if resp.Data.MatchResult != nil {
		t.Fatalf("rejected query still returned roster data: %s", body)
	}
	// Belt and braces: whatever the error text says, no address may appear in it.
	for _, email := range []string{match.userA.Email, match.userB.Email} {
		if email != "" && strings.Contains(string(body), email) {
			t.Fatalf("response leaked participant email %q: %s", email, body)
		}
	}
}
