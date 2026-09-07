package graph

import (
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gorilla/websocket"
	"github.com/scruffyprodigy/joinquest/graph/generated"
	"github.com/scruffyprodigy/joinquest/internal/auth"
)

// NewGraphQLServer builds the production GraphQL handler (HTTP + graphql-ws).
// main and integration tests must use this so WebSocket origin and auth stay consistent.
func NewGraphQLServer(signer *auth.Signer, apiKeys auth.DeveloperAPIKeyVerifier, resolver *Resolver) *handler.Server {
	gql := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	gql.AddTransport(transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: auth.WebSocketOriginAllowed,
		},
		KeepAlivePingInterval: 10 * time.Second,
		InitFunc:              WebsocketInitFunc(signer, apiKeys),
	})
	gql.AddTransport(transport.Options{})
	gql.AddTransport(transport.GET{})
	gql.AddTransport(transport.POST{})
	return gql
}
