package graph

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/scruffyprodigy/joinquest/graph/generated"
)

func TestMeResolver(t *testing.T) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	var resp struct {
		Me *struct {
			ID          string
			Email       *string
			DisplayName *string
			CreatedAt   string
		}
	}

	err := c.Post(`query { 
		me { 
			id 
			email 
			displayName 
			createdAt 
		} 
	}`, &resp)
	if err != nil {
		t.Fatalf("GraphQL query failed: %v", err)
	}

	if resp.Me != nil {
		t.Errorf("expected unauthenticated me to be null, got %+v", resp.Me)
	}
}

func TestGamesResolver(t *testing.T) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	var resp struct {
		Games []struct {
			ID        string
			Name      string
			CreatedAt string
		}
	}

	err := c.Post(`query { 
		games { 
			id 
			name 
			createdAt 
		} 
	}`, &resp)
	if err == nil {
		t.Fatal("expected games query to fail without a configured store")
	}
}

func TestGamesWithPagination(t *testing.T) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	err := c.Post(`query { 
		games(limit: 1) { 
			id 
			name 
		} 
	}`, &struct{}{})
	if err == nil {
		t.Fatal("expected games query to fail without a configured store")
	}
}

func TestGoodsResolverRequiresStore(t *testing.T) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	err := c.Post(`query { 
		goods { 
			id 
			code 
			name 
		} 
	}`, &struct{}{})
	if err == nil {
		t.Fatal("expected goods query to fail without a configured store")
	}
}

func TestMyInventoryResolverRequiresAuthAndStore(t *testing.T) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	err := c.Post(`query { 
		myInventory { 
			good { 
				id 
			} 
			quantity 
		} 
	}`, &struct{}{})
	if err == nil {
		t.Fatal("expected myInventory to fail without auth and store")
	}
}

func TestJoinQueueMutationRequiresAuthAndStore(t *testing.T) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	err := c.Post(`mutation { 
		joinQueue(queueId: "test-queue-id") { 
			queued 
		} 
	}`, &struct{}{})
	if err == nil {
		t.Fatal("expected joinQueue to fail without auth and store")
	}
}

func TestGameNotFound(t *testing.T) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	err := c.Post(`query { 
		game(id: "00000000-0000-4000-8000-000000000099") { 
			id 
			name 
		} 
	}`, &struct{}{})

	if err == nil {
		t.Fatal("expected game query to fail without a configured store")
	}
}

// Test direct resolver calls
func TestDirectResolverCalls(t *testing.T) {
	resolver := &Resolver{}
	queryResolver := resolver.Query()

	// Test direct healthz call
	healthz, err := queryResolver.Healthz(context.Background())
	if err != nil {
		t.Fatalf("Direct healthz call failed: %v", err)
	}
	if healthz != "ok" {
		t.Errorf("Expected healthz to return 'ok', got %q", healthz)
	}

	// Test direct version call
	version, err := queryResolver.Version(context.Background())
	if err != nil {
		t.Fatalf("Direct version call failed: %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("Expected version to return '1.0.0', got %q", version)
	}
}
