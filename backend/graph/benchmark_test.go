package graph

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/scruffyprodigy/joinquest/graph/generated"
)

func BenchmarkHealthz(b *testing.B) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp struct {
			Healthz string
		}
		err := c.Post(`query { healthz }`, &resp)
		if err != nil {
			b.Fatalf("GraphQL query failed: %v", err)
		}
	}
}

func BenchmarkHealthzDirect(b *testing.B) {
	resolver := &Resolver{}
	queryResolver := resolver.Query()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := queryResolver.Healthz(context.Background())
		if err != nil {
			b.Fatalf("Direct healthz call failed: %v", err)
		}
	}
}

func BenchmarkVersion(b *testing.B) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp struct {
			Version string
		}
		err := c.Post(`query { version }`, &resp)
		if err != nil {
			b.Fatalf("GraphQL query failed: %v", err)
		}
	}
}

func BenchmarkMe(b *testing.B) {
	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	c := client.New(srv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp struct {
			Me struct {
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
			b.Fatalf("GraphQL query failed: %v", err)
		}
	}
}
