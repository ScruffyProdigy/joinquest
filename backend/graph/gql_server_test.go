package graph

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/scruffyprodigy/joinquest/graph/generated"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/pubsub"
)

func TestGraphQLServerWebSocketAllowsLoopbackOrigin(t *testing.T) {
	signer, bearer := testSignerAndBearer(t)

	gql := NewGraphQLServer(signer, nil, NewResolver(nil, nil, pubsub.NewMemory()))
	srv := httptest.NewServer(auth.Middleware(signer, nil, gql))
	defer srv.Close()

	connectGraphQLWS(t, graphQLWSURL(srv.URL), "http://127.0.0.1:5173", bearer)
}

func TestGraphQLServerWebSocketRejectsForeignOrigin(t *testing.T) {
	signer, bearer := testSignerAndBearer(t)

	gql := NewGraphQLServer(signer, nil, NewResolver(nil, nil, pubsub.NewMemory()))
	srv := httptest.NewServer(auth.Middleware(signer, nil, gql))
	defer srv.Close()

	// Sanity: production wiring accepts loopback.
	connectGraphQLWS(t, graphQLWSURL(srv.URL), "http://localhost:5173", bearer)
	_ = bearer

	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", graphqlWSSubprotocol)
	header.Set("Origin", "https://evil.example")
	_, _, err := websocket.DefaultDialer.Dial(graphQLWSURL(srv.URL), header)
	if err == nil {
		t.Fatal("expected foreign origin to be rejected")
	}
}

// TestNewDefaultServerWebSocketPreventsLoopbackOriginFix documents a production
// regression: handler.NewDefaultServer registers a WebSocket transport first, so a
// later AddTransport with WebSocketOriginAllowed never runs (first match wins).
func TestNewDefaultServerWebSocketPreventsLoopbackOriginFix(t *testing.T) {
	signer, _ := testSignerAndBearer(t)
	resolver := NewResolver(nil, nil, pubsub.NewMemory())

	broken := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	broken.AddTransport(transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: auth.WebSocketOriginAllowed,
		},
		InitFunc: WebsocketInitFunc(signer, nil),
	})

	srv := httptest.NewServer(auth.Middleware(signer, nil, broken))
	defer srv.Close()

	header := http.Header{}
	header.Set("Sec-WebSocket-Protocol", graphqlWSSubprotocol)
	header.Set("Origin", "http://localhost:5173")
	_, resp, err := websocket.DefaultDialer.Dial(graphQLWSURL(srv.URL), header)
	if err == nil {
		t.Fatal("expected broken duplicate-default-server wiring to reject or fail loopback origin")
	}
	if resp != nil && resp.StatusCode == http.StatusOK {
		t.Fatalf("unexpected successful upgrade with duplicate default websocket transport")
	}
}

func testSignerAndBearer(t *testing.T) (*auth.Signer, string) {
	t.Helper()
	signer, err := auth.LoadSignerFromEnv()
	if err != nil {
		t.Fatalf("load signer: %v", err)
	}
	token, err := signer.SignUserToken(uuid.MustParse("00000000-0000-4000-8000-000000000099"), time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signer, "Bearer " + token
}
