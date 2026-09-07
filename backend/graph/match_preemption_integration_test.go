package graph

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

// recordingEmitter captures metric signals so a test can assert one was emitted.
type recordingEmitter struct {
	mu     sync.Mutex
	counts map[string]float64
}

func newRecordingEmitter() *recordingEmitter {
	return &recordingEmitter{counts: map[string]float64{}}
}

func (e *recordingEmitter) Gauge(name string, value float64, _ map[string]string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.counts[name] += value
}

func (e *recordingEmitter) Count(name string, value float64, _ map[string]string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.counts[name] += value
}

func (e *recordingEmitter) Close() error { return nil }

func (e *recordingEmitter) total(name string) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.counts[name]
}

// A completion that arrives after something else already ended the session used to be
// swallowed: CompleteSession returned ErrNotFound, the resolver ignored it and answered
// true, and the game server was told the result landed with nothing recording that the
// table reset never ran. It stays a success — the result itself is recorded — but it
// must leave a signal behind (JQ-171).
func TestReportMatchResultSignalsPreemptedCompletion(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "preemption-pepper")

	emitter := newRecordingEmitter()
	env.resolver.Emitter = emitter

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

	// Pre-empt the resolver: something else completes the session first.
	if err := env.Store.CompleteSession(ctx, sessionID, time.Now()); err != nil {
		t.Fatalf("CompleteSession: %v", err)
	}

	gameID := uuid.MustParse(store.DemoPrimaryGameIDStr)
	serviceToken, err := auth.FormatGameServiceToken(gameID)
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}

	mutation := `mutation Report($matchId: ID!, $status: MatchResultStatus!) {
		reportMatchResult(matchId: $matchId, status: $status)
	}`
	reportBody := postGraphQLWithBearer(t, env.Handler, serviceToken, mutation, map[string]any{
		"matchId": matchID,
		"status":  "COMPLETED",
	})
	var reportResp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data struct {
			ReportMatchResult bool `json:"reportMatchResult"`
		} `json:"data"`
	}
	if err := json.Unmarshal(reportBody, &reportResp); err != nil {
		t.Fatalf("decode report: %v body=%s", err, reportBody)
	}
	if len(reportResp.Errors) > 0 {
		t.Fatalf("reportMatchResult errors: %+v", reportResp.Errors)
	}
	// The result itself was recorded, so the game server must not be told to retry.
	if !reportResp.Data.ReportMatchResult {
		t.Fatalf("reportMatchResult = false, body=%s", reportBody)
	}

	if got := emitter.total(metricCompletionPreempted); got != 1 {
		t.Fatalf("%s = %v, want 1: the pre-empted completion was swallowed", metricCompletionPreempted, got)
	}
}

// The ordinary path completes the session itself, so it must not raise the pre-emption
// signal — otherwise the metric is noise and nobody will act on it.
func TestReportMatchResultEmitsNoSignalOnNormalCompletion(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	t.Setenv("LOBBY_ISSUER_URL", "http://localhost:8080")
	t.Setenv("LOBBY_PUBLIC_URL", "http://localhost:5173")
	t.Setenv("LOBBY_GAME_TOKEN_PEPPER", "preemption-pepper")

	emitter := newRecordingEmitter()
	env.resolver.Emitter = emitter

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

	gameID := uuid.MustParse(store.DemoPrimaryGameIDStr)
	serviceToken, err := auth.FormatGameServiceToken(gameID)
	if err != nil {
		t.Fatalf("FormatGameServiceToken: %v", err)
	}

	mutation := `mutation Report($matchId: ID!, $status: MatchResultStatus!) {
		reportMatchResult(matchId: $matchId, status: $status)
	}`
	postGraphQLWithBearer(t, env.Handler, serviceToken, mutation, map[string]any{
		"matchId": matchID,
		"status":  "COMPLETED",
	})

	if got := emitter.total(metricCompletionPreempted); got != 0 {
		t.Fatalf("%s = %v, want 0 on the ordinary path", metricCompletionPreempted, got)
	}
}
