package graph

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/scruffyprodigy/playhub/internal/auth"
	"github.com/scruffyprodigy/playhub/internal/formingworker"
	"github.com/scruffyprodigy/playhub/internal/gameclient"
	"github.com/scruffyprodigy/playhub/internal/pubsub"
	"github.com/scruffyprodigy/playhub/internal/spiritanimal"
	"github.com/scruffyprodigy/playhub/internal/store"
	"github.com/scruffyprodigy/playhub/internal/testdb"
	_ "github.com/lib/pq"
)

const (
	demoDefaultQueueID = store.DemoDefaultQueueIDStr
	// Must match gqlgen default and frontend graphql-ws client (not graphql-transport-ws).
	graphqlWSSubprotocol = "graphql-ws"
)

type queueIntegrationEnv struct {
	Server   *httptest.Server
	Client   *client.Client
	Handler  http.Handler
	DB       *sql.DB
	Store    *store.Store
	Signer   *auth.Signer
	resolver *Resolver
}

func newQueueIntegrationEnv(t *testing.T) *queueIntegrationEnv {
	t.Helper()

	databaseURL := testdb.RequireURL(t)

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st := store.New(db)
	signer, err := auth.LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}

	authService, err := auth.NewService(st, signer)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	// Integration tests share the seeded demo game row; restore handoff URLs so we never
	// persist httptest ports (e.g. :50624) into api_base_url for local dev.
	ctx := context.Background()
	if err := st.RestorePrimaryGameHandoffURLs(ctx); err != nil {
		t.Fatalf("restore demo game handoff urls: %v", err)
	}
	t.Cleanup(func() { _ = st.RestorePrimaryGameHandoffURLs(context.Background()) })

	resolver := NewResolver(st, authService, pubsub.NewMemory())
	resolver.SpiritAnimal = spiritanimal.NewRunnerFromEnv(st, "https://joinquest.test")
	resolver.FormingWorker = formingworker.New(st, resolver.HandleFormingReconciled, 5*time.Millisecond, time.Minute)
	resolver.FormingWorker.SetProvisionHook(resolver.HandleUnprovisionedSession)
	t.Cleanup(func() { resolver.FormingWorker.Shutdown() })
	env := &queueIntegrationEnv{
		DB:       db,
		Store:    st,
		Signer:   signer,
		resolver: resolver,
	}
	env.rebuildHTTPServer(t)
	return env
}

func (env *queueIntegrationEnv) rebuildHTTPServer(t *testing.T) {
	t.Helper()
	gql := NewGraphQLServer(env.Signer, env.Store, env.resolver)
	gqlHandler := auth.Middleware(env.Signer, env.Store, gql)
	mux := http.NewServeMux()
	mux.Handle("/", gqlHandler)
	mux.Handle("/graphql", gqlHandler)
	handler := auth.CORSMiddleware(mux)

	if env.Server != nil {
		env.Server.Close()
	}
	env.Server = httptest.NewServer(handler)
	t.Cleanup(env.Server.Close)
	env.Handler = gqlHandler
	env.Client = client.New(gqlHandler)
}

func (env *queueIntegrationEnv) newCleaner(t *testing.T) *store.TestCleaner {
	return env.Store.NewTestCleaner(t)
}

func (env *queueIntegrationEnv) resolverWithProvisioner(t *testing.T, provisioner gameclient.MatchProvisioner) {
	t.Helper()
	env.resolver.GameProvisioner = provisioner
	env.rebuildHTTPServer(t)
}

func createTestUserSession(t *testing.T, ctx context.Context, env *queueIntegrationEnv, cleaner *store.TestCleaner) (bearer string, cookie *http.Cookie) {
	t.Helper()

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "queue-ws-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)

	token, err := env.Signer.SignUserToken(user.ID, time.Hour)
	if err != nil {
		t.Fatalf("SignUserToken: %v", err)
	}

	cfg := auth.CookieConfigFromEnv()
	return "Bearer " + token, &http.Cookie{Name: cfg.Name, Value: token}
}

func createTestUserSessionForUser(t *testing.T, env *queueIntegrationEnv, userID uuid.UUID) (bearer string, cookie *http.Cookie) {
	t.Helper()

	token, err := env.Signer.SignUserToken(userID, time.Hour)
	if err != nil {
		t.Fatalf("SignUserToken: %v", err)
	}

	cfg := auth.CookieConfigFromEnv()
	return "Bearer " + token, &http.Cookie{Name: cfg.Name, Value: token}
}

func graphQLWSURL(httpURL string) string {
	u, _ := url.Parse(httpURL)
	u.Scheme = "ws"
	u.Path = "/graphql"
	return u.String()
}

func writeGraphQLWS(conn *websocket.Conn, payload any) error {
	return conn.WriteJSON(payload)
}

func readGraphQLWS(conn *websocket.Conn) (map[string]any, error) {
	var msg map[string]any
	if err := conn.ReadJSON(&msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func waitForGraphQLWSType(conn *websocket.Conn, wantType string, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg, err := readGraphQLWS(conn)
		if err != nil {
			return nil, err
		}
		typ, _ := msg["type"].(string)
		switch typ {
		case "ping":
			if err := writeGraphQLWS(conn, map[string]any{"type": "pong"}); err != nil {
				return nil, err
			}
		case "ka":
			// graphql-ws keep-alive; no response required
		case wantType:
			return msg, nil
		case "connection_error", "error":
			return nil, fmt.Errorf("graphql websocket error: %v", msg)
		}
	}
	return nil, fmt.Errorf("timed out waiting for websocket message type %q", wantType)
}

func connectGraphQLWS(t *testing.T, wsURL, origin, bearer string) *websocket.Conn {
	t.Helper()

	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", graphqlWSSubprotocol)
	if origin != "" {
		header.Set("Origin", origin)
	}

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			t.Fatalf("websocket dial failed: %v (HTTP %d)", err, resp.StatusCode)
		}
		t.Fatalf("websocket dial failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	initPayload := map[string]any{}
	if bearer != "" {
		initPayload["Authorization"] = bearer
	}
	if err := writeGraphQLWS(conn, map[string]any{
		"type":    "connection_init",
		"payload": initPayload,
	}); err != nil {
		t.Fatalf("connection_init: %v", err)
	}

	if _, err := waitForGraphQLWSType(conn, "connection_ack", 5*time.Second); err != nil {
		t.Fatalf("connection_ack: %v", err)
	}

	return conn
}

func subscribeQueueUpdated(t *testing.T, conn *websocket.Conn, queueID string) string {
	t.Helper()

	subID := "sub-" + uuid.NewString()
	query := fmt.Sprintf(`subscription { queueUpdated(queueId: %q) { status joinUrl queuedCount } }`, queueID)
	if err := writeGraphQLWS(conn, map[string]any{
		"id":   subID,
		"type": "start",
		"payload": map[string]any{
			"query": query,
		},
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	return subID
}

func nextQueueUpdatePayload(t *testing.T, conn *websocket.Conn, subID string, timeout time.Duration) map[string]any {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg, err := readGraphQLWS(conn)
		if err != nil {
			t.Fatalf("read websocket: %v", err)
		}
		typ, _ := msg["type"].(string)
		if typ == "ping" {
			_ = writeGraphQLWS(conn, map[string]any{"type": "pong"})
			continue
		}
		if typ == "ka" {
			continue
		}
		if typ == "error" || typ == "complete" {
			t.Fatalf("subscription ended: %v", msg)
		}
		if typ != "data" || msg["id"] != subID {
			continue
		}
		payload, _ := msg["payload"].(map[string]any)
		data, _ := payload["data"].(map[string]any)
		update, _ := data["queueUpdated"].(map[string]any)
		if update != nil {
			return update
		}
	}
	t.Fatalf("timed out waiting for queueUpdated event")
	return nil
}

func postGraphQL(t *testing.T, handler http.Handler, query string, variables map[string]any, cookies ...*http.Cookie) []byte {
	t.Helper()

	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code >= http.StatusBadRequest {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func TestWebSocketUpgradeAllowsLoopbackOrigin(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	bearer, _ := createTestUserSession(t, context.Background(), env, cleaner)

	connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://127.0.0.1:5173", bearer)
}

func TestWebSocketUpgradeRejectsForeignOrigin(t *testing.T) {
	env := newQueueIntegrationEnv(t)

	header := http.Header{}
	header.Set("Origin", "https://not-lobby.example")
	_, _, err := websocket.DefaultDialer.Dial(graphQLWSURL(env.Server.URL), header)
	if err == nil {
		t.Fatal("expected websocket upgrade to fail for foreign origin")
	}
}

func TestWebSocketRequiresAuthentication(t *testing.T) {
	env := newQueueIntegrationEnv(t)

	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", graphqlWSSubprotocol)
	header.Set("Origin", "http://localhost:5173")
	conn, _, err := websocket.DefaultDialer.Dial(graphQLWSURL(env.Server.URL), header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	_ = writeGraphQLWS(conn, map[string]any{"type": "connection_init", "payload": map[string]any{}})
	_, err = waitForGraphQLWSType(conn, "connection_ack", 3*time.Second)
	if err == nil {
		t.Fatal("expected connection_init without auth to fail")
	}
}

func TestJoinQueueGraphQLAlwaysReturnsQueued(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	clearDemoQueue(t, env.Store)
	_, cookie := createTestUserSession(t, context.Background(), env, cleaner)

	var resp struct {
		JoinQueue struct {
			Queued      bool    `json:"queued"`
			JoinURL     *string `json:"joinUrl"`
			SessionID   *string `json:"sessionId"`
			QueuedCount *int    `json:"queuedCount"`
		} `json:"joinQueue"`
	}
	if err := env.Client.Post(`mutation Join($id: ID!) {
		joinQueue(queueId: $id) { queued joinUrl sessionId queuedCount }
	}`, &resp, client.AddCookie(cookie), client.Var("id", demoDefaultQueueID)); err != nil {
		t.Fatalf("joinQueue: %v", err)
	}
	if !resp.JoinQueue.Queued {
		t.Fatalf("expected queued=true for solo join, got %+v", resp.JoinQueue)
	}
	if resp.JoinQueue.JoinURL != nil || resp.JoinQueue.SessionID != nil {
		t.Fatalf("solo join must not return match fields, got %+v", resp.JoinQueue)
	}
}

func TestJoinQueueGraphQLMatchArrivesViaWorker(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)
	env.resolverWithProvisioner(t, &syncProvisioner{})

	_, cookieA := createTestUserSession(t, ctx, env, cleaner)
	_, cookieB := createTestUserSession(t, ctx, env, cleaner)

	joinMutation := `mutation Join($id: ID!) {
		joinQueue(queueId: $id) { queued joinUrl sessionId queuedCount }
	}`
	vars := client.Var("id", demoDefaultQueueID)

	var waitResp struct {
		JoinQueue struct {
			Queued      bool    `json:"queued"`
			JoinURL     *string `json:"joinUrl"`
			SessionID   *string `json:"sessionId"`
			QueuedCount *int    `json:"queuedCount"`
		} `json:"joinQueue"`
	}
	if err := env.Client.Post(joinMutation, &waitResp, client.AddCookie(cookieA), vars); err != nil {
		t.Fatalf("join A: %v", err)
	}
	if !waitResp.JoinQueue.Queued || waitResp.JoinQueue.JoinURL != nil {
		t.Fatalf("expected A waiting only, got %+v", waitResp.JoinQueue)
	}

	var matchResp struct {
		JoinQueue struct {
			Queued      bool    `json:"queued"`
			JoinURL     *string `json:"joinUrl"`
			SessionID   *string `json:"sessionId"`
			QueuedCount *int    `json:"queuedCount"`
		} `json:"joinQueue"`
	}
	if err := env.Client.Post(joinMutation, &matchResp, client.AddCookie(cookieB), vars); err != nil {
		t.Fatalf("join B: %v", err)
	}
	if !matchResp.JoinQueue.Queued || matchResp.JoinQueue.JoinURL != nil {
		t.Fatalf("expected B queued from mutation, got %+v", matchResp.JoinQueue)
	}

	flushFormingWorker(t, env, ctx, uuid.MustParse(demoDefaultQueueID))

	var statusResp struct {
		MyQueueStatus struct {
			Queued    bool    `json:"queued"`
			JoinURL   *string `json:"joinUrl"`
			SessionID *string `json:"sessionId"`
		} `json:"myQueueStatus"`
	}
	statusQuery := `query Status($id: ID!) { myQueueStatus(queueId: $id) { queued joinUrl sessionId } }`
	if err := env.Client.Post(statusQuery, &statusResp, client.AddCookie(cookieB), vars); err != nil {
		t.Fatalf("myQueueStatus: %v", err)
	}
	if statusResp.MyQueueStatus.SessionID == nil || statusResp.MyQueueStatus.JoinURL == nil {
		t.Fatalf("expected B matched via worker, got %+v", statusResp.MyQueueStatus)
	}
}

func clearDemoQueue(t *testing.T, st *store.Store) {
	t.Helper()
	queueID, err := uuid.Parse(demoDefaultQueueID)
	if err != nil {
		t.Fatalf("parse demo queue id: %v", err)
	}
	if err := st.ResetModeQueueIntegrationState(context.Background(), queueID); err != nil {
		t.Fatalf("reset demo queue: %v", err)
	}
}

func flushFormingWorker(t *testing.T, env *queueIntegrationEnv, ctx context.Context, queueID uuid.UUID) {
	t.Helper()
	env.resolver.FormingWorker.CancelPending(queueID)
	for i := 0; i < 3; i++ {
		if err := env.resolver.FormingWorker.ReconcileNow(ctx, queueID); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		if i < 2 {
			time.Sleep(15 * time.Millisecond)
			env.resolver.FormingWorker.CancelPending(queueID)
		}
	}
}

func waitForProvisionCalls(t *testing.T, provisioner *syncProvisioner, want int) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		provisioner.mu.Lock()
		n := len(provisioner.calls)
		provisioner.mu.Unlock()
		if n >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	provisioner.mu.Lock()
	n := len(provisioner.calls)
	provisioner.mu.Unlock()
	t.Fatalf("expected >= %d provision calls, got %d", want, n)
}

func TestQueueSubscriptionNotifiesWaitingPlayerOnMatch(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()
	clearDemoQueue(t, env.Store)

	env.resolverWithProvisioner(t, &syncProvisioner{})

	bearerA, cookieA := createTestUserSession(t, ctx, env, cleaner)
	_, cookieB := createTestUserSession(t, ctx, env, cleaner)

	joinQuery := `mutation Join($id: ID!) { joinQueue(queueId: $id) { queued queuedCount } }`
	vars := map[string]any{"id": demoDefaultQueueID}

	conn := connectGraphQLWS(t, graphQLWSURL(env.Server.URL), "http://localhost:5173", bearerA)
	subID := subscribeQueueUpdated(t, conn, demoDefaultQueueID)

	postGraphQL(t, env.Handler, joinQuery, vars, cookieA)

	first := nextQueueUpdatePayload(t, conn, subID, 5*time.Second)
	if first["status"] != "WAITING" {
		t.Fatalf("expected WAITING after A joined, got %+v (stale queue rows? run db.sh clean-test-data)", first)
	}

	postGraphQL(t, env.Handler, joinQuery, vars, cookieB)

	matched := nextQueueUpdatePayload(t, conn, subID, 5*time.Second)
	if matched["status"] != "MATCHED" {
		t.Fatalf("expected MATCHED over websocket, got %+v", matched)
	}
	joinURL, _ := matched["joinUrl"].(string)
	if joinURL == "" {
		t.Fatalf("expected joinUrl on MATCHED event, got %+v", matched)
	}
	if !strings.Contains(joinURL, "match=") || !strings.Contains(joinURL, "token=") {
		t.Fatalf("expected protocol handoff launch URL (?match=&token=), got %q", joinURL)
	}
}

func TestSubscriptionAuthReturnsBearerToken(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	bearer, cookie := createTestUserSession(t, context.Background(), env, cleaner)

	var resp struct {
		SubscriptionAuth *string `json:"subscriptionAuth"`
	}
	if err := env.Client.Post(`query { subscriptionAuth }`, &resp, client.AddCookie(cookie)); err != nil {
		t.Fatalf("subscriptionAuth: %v", err)
	}
	if resp.SubscriptionAuth == nil || *resp.SubscriptionAuth != bearer {
		t.Fatalf("expected subscriptionAuth %q, got %+v", bearer, resp.SubscriptionAuth)
	}
}
