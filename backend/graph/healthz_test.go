package graph

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/scruffyprodigy/joinquest/graph/generated"
)

func TestHealthz(t *testing.T) {
	// Create a new GraphQL handler with our resolver
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	// Test the healthz query
	var resp struct {
		Healthz string
	}

	err := c.Post(`query { healthz }`, &resp)
	if err != nil {
		t.Fatalf("GraphQL query failed: %v", err)
	}

	// Verify the response
	expected := "ok"
	if resp.Healthz != expected {
		t.Errorf("Expected healthz to return %q, got %q", expected, resp.Healthz)
	}
}

func TestHealthzDirect(t *testing.T) {
	// Test the resolver directly without GraphQL layer
	resolver := &Resolver{}
	queryResolver := resolver.Query()

	// Call the healthz resolver directly
	result, err := queryResolver.Healthz(context.Background())
	if err != nil {
		t.Fatalf("Healthz resolver failed: %v", err)
	}

	// Verify the result
	expected := "ok"
	if result != expected {
		t.Errorf("Expected healthz to return %q, got %q", expected, result)
	}
}

func TestVersion(t *testing.T) {
	// Test the version query as well since it's similar
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	var resp struct {
		Version string
	}

	err := c.Post(`query { version }`, &resp)
	if err != nil {
		t.Fatalf("GraphQL query failed: %v", err)
	}

	// Verify the response
	expected := "1.0.0"
	if resp.Version != expected {
		t.Errorf("Expected version to return %q, got %q", expected, resp.Version)
	}
}

func TestMultipleQueries(t *testing.T) {
	// Test multiple queries in a single request
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	var resp struct {
		Healthz string
		Version string
	}

	err := c.Post(`query { 
		healthz 
		version 
	}`, &resp)
	if err != nil {
		t.Fatalf("GraphQL query failed: %v", err)
	}

	// Verify both responses
	if resp.Healthz != "ok" {
		t.Errorf("Expected healthz to return 'ok', got %q", resp.Healthz)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("Expected version to return '1.0.0', got %q", resp.Version)
	}
}
