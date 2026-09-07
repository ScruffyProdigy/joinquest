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

	match := seedActiveMatch(t, env, cleaner)
	reportPlayerFinished(t, env, match.sessionID, match.userA.ID, 1)
	reportMatchResult(t, env, match.sessionID, "COMPLETED", match.userA.ID.String())
	return match
}

// seedActiveMatch stops one step earlier than seedFinishedMatch: a provisioned, still
// active two-player demo match with nobody reported finished and no result recorded, so a
// test can drive the lifecycle mutations itself in whatever order it needs.
func seedActiveMatch(t *testing.T, env *queueIntegrationEnv, cleaner *store.TestCleaner) finishedMatch {
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

	return finishedMatch{sessionID: sessionID, userA: userA, userB: userB, cookieA: cookieA, cookieB: cookieB}
}

// demoGameServiceToken is the bearer a game server presents on the lifecycle mutations.
func demoGameServiceToken(t *testing.T) string {
	t.Helper()
	token, err := auth.FormatGameServiceToken(uuid.MustParse(store.DemoPrimaryGameIDStr))
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}
	return token
}

func reportPlayerFinished(t *testing.T, env *queueIntegrationEnv, sessionID, userID uuid.UUID, placement int) {
	t.Helper()
	mutation := `mutation Finish($matchId: ID!, $lobbyUserId: ID!, $reason: PlayerFinishReason!, $placement: Int) {
		reportPlayerFinished(matchId: $matchId, lobbyUserId: $lobbyUserId, reason: $reason, placement: $placement)
	}`
	requireNoGraphQLErrors(t, postGraphQLWithBearer(t, env.Handler, demoGameServiceToken(t), mutation, map[string]any{
		"matchId":     sessionID.String(),
		"lobbyUserId": userID.String(),
		"reason":      "COMPLETED",
		"placement":   placement,
	}))
}

func reportMatchResult(t *testing.T, env *queueIntegrationEnv, sessionID uuid.UUID, status string, winnerIDs ...string) {
	t.Helper()
	mutation := `mutation Report($matchId: ID!, $status: MatchResultStatus!, $winnerLobbyUserIds: [ID!]) {
		reportMatchResult(matchId: $matchId, status: $status, winnerLobbyUserIds: $winnerLobbyUserIds)
	}`
	requireNoGraphQLErrors(t, postGraphQLWithBearer(t, env.Handler, demoGameServiceToken(t), mutation, map[string]any{
		"matchId":            sessionID.String(),
		"status":             status,
		"winnerLobbyUserIds": winnerIDs,
	}))
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

const playAgainMutation = `mutation PlayAgain($matchId: ID!) {
	playAgain(matchId: $matchId) {
		table { id }
		inviteCode
		seated
	}
}`

const declinePlayAgainMutation = `mutation Decline($matchId: ID!) {
	declinePlayAgain(matchId: $matchId) {
		path
		kind
	}
}`

type playAgainResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Data struct {
		PlayAgain *struct {
			Table struct {
				ID string `json:"id"`
			} `json:"table"`
			InviteCode string `json:"inviteCode"`
			Seated     bool   `json:"seated"`
		} `json:"playAgain"`
	} `json:"data"`
}

type declinePlayAgainResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Data struct {
		DeclinePlayAgain *struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"declinePlayAgain"`
	} `json:"data"`
}

func playAgain(t *testing.T, env *queueIntegrationEnv, sessionID uuid.UUID, cookie *http.Cookie) playAgainResponse {
	t.Helper()
	body := postGraphQL(t, env.Handler, playAgainMutation, map[string]any{"matchId": sessionID.String()}, cookie)
	var resp playAgainResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode playAgain: %v body=%s", err, body)
	}
	return resp
}

func declinePlayAgain(t *testing.T, env *queueIntegrationEnv, sessionID uuid.UUID, cookie *http.Cookie) declinePlayAgainResponse {
	t.Helper()
	body := postGraphQL(t, env.Handler, declinePlayAgainMutation, map[string]any{"matchId": sessionID.String()}, cookie)
	var resp declinePlayAgainResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode declinePlayAgain: %v body=%s", err, body)
	}
	return resp
}

// TestPlayAgainSeatsBothPlayersAtOneTable is the product guarantee: every player who accepts
// play-again lands at the SAME table, never one table each. This is the assertion that would
// catch a naive "create a table per caller" implementation — it fails unless the second
// caller's claim resolves to the first caller's table id.
func TestPlayAgainSeatsBothPlayersAtOneTable(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	first := playAgain(t, env, match.sessionID, match.cookieA)
	if len(first.Errors) > 0 || first.Data.PlayAgain == nil {
		t.Fatalf("PlayAgain A: %+v", first.Errors)
	}
	second := playAgain(t, env, match.sessionID, match.cookieB)
	if len(second.Errors) > 0 || second.Data.PlayAgain == nil {
		t.Fatalf("PlayAgain B: %+v", second.Errors)
	}

	if first.Data.PlayAgain.Table.ID != second.Data.PlayAgain.Table.ID {
		t.Fatalf("players landed on different tables: %s vs %s",
			first.Data.PlayAgain.Table.ID, second.Data.PlayAgain.Table.ID)
	}
	if first.Data.PlayAgain.InviteCode == "" {
		t.Error("expected an invite code to route to")
	}
	if first.Data.PlayAgain.InviteCode != second.Data.PlayAgain.InviteCode {
		t.Errorf("invite codes differ: %q vs %q", first.Data.PlayAgain.InviteCode, second.Data.PlayAgain.InviteCode)
	}
	if !first.Data.PlayAgain.Seated {
		t.Error("player A expected to be seated at the regroup table")
	}
	if !second.Data.PlayAgain.Seated {
		t.Error("player B expected to be seated at the regroup table")
	}

	// The roster read side (Task 6) must agree: both callers read IN.
	result := queryMatchResult(t, env, match.sessionID, match.cookieA)
	if len(result.Errors) > 0 {
		t.Fatalf("matchResult errors: %+v", result.Errors)
	}
	states := map[string]string{}
	for _, p := range result.Data.MatchResult.Participants {
		states[p.User.ID] = p.Regroup
	}
	if states[match.userA.ID.String()] != "IN" {
		t.Errorf("player A regroup = %q, want IN", states[match.userA.ID.String()])
	}
	if states[match.userB.ID.String()] != "IN" {
		t.Errorf("player B regroup = %q, want IN", states[match.userB.ID.String()])
	}
}

// TestDeclinePlayAgainMarksOut covers the decline path: the caller's roster entry reads OUT
// afterward, and the mutation itself resolves to a usable return destination.
func TestDeclinePlayAgainMarksOut(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)

	resp := declinePlayAgain(t, env, match.sessionID, match.cookieA)
	if len(resp.Errors) > 0 {
		t.Fatalf("DeclinePlayAgain: %+v", resp.Errors)
	}
	if resp.Data.DeclinePlayAgain == nil {
		t.Fatal("declinePlayAgain returned a null destination")
	}

	got := queryMatchResult(t, env, match.sessionID, match.cookieA)
	if len(got.Errors) > 0 {
		t.Fatalf("matchResult errors: %+v", got.Errors)
	}
	for _, p := range got.Data.MatchResult.Participants {
		if p.User.ID == match.userA.ID.String() && p.Regroup != "OUT" {
			t.Fatalf("userA regroup = %q, want OUT", p.Regroup)
		}
	}
}

// TestPlayAgainAndDeclinePlayAgainRefuseNonParticipant is the authorization gate: match ids
// travel in a player-editable return URL, so a signed-in stranger who is not in the match must
// be refused by both write mutations, exactly like the read side.
func TestPlayAgainAndDeclinePlayAgainRefuseNonParticipant(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	match := seedFinishedMatch(t, env, cleaner)
	_, outsiderCookie := createTestUserSession(t, context.Background(), env, cleaner)

	playResp := playAgain(t, env, match.sessionID, outsiderCookie)
	if len(playResp.Errors) == 0 {
		t.Fatal("expected playAgain to refuse a non-participant")
	}
	if playResp.Data.PlayAgain != nil {
		t.Fatalf("non-participant received a table: %+v", playResp.Data.PlayAgain)
	}

	declineResp := declinePlayAgain(t, env, match.sessionID, outsiderCookie)
	if len(declineResp.Errors) == 0 {
		t.Fatal("expected declinePlayAgain to refuse a non-participant")
	}
	if declineResp.Data.DeclinePlayAgain != nil {
		t.Fatalf("non-participant received a destination: %+v", declineResp.Data.DeclinePlayAgain)
	}
}

// TestPlayAgainRefusesUnfinishedMatch pins ErrSessionNotFinished's distinct surfacing: a real
// participant in a match that has not completed yet gets an error that is NOT "you did not
// play in this match" — the client needs to tell "too early" apart from "not your match" so it
// can say something true instead of a generic failure.
func TestPlayAgainRefusesUnfinishedMatch(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "play-again-unfinished-pepper")

	provisioner := &syncProvisioner{}
	env.resolverWithProvisioner(t, provisioner)

	_, cookieA := createTestUserSession(t, ctx, env, cleaner)
	_, cookieB := createTestUserSession(t, ctx, env, cleaner)

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

	resp := playAgain(t, env, sessionID, cookieA)
	if len(resp.Errors) == 0 {
		t.Fatal("expected playAgain to refuse a still-running match")
	}
	for _, e := range resp.Errors {
		if strings.Contains(e.Message, "did not play") {
			t.Fatalf("still-running match reported as non-participant, want a distinct message: %q", e.Message)
		}
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
