package graph

import (
	"context"
	"testing"

	"github.com/scruffyprodigy/joinquest/graph/model"
)

func TestUserIsAdminResolver(t *testing.T) {
	t.Setenv("LOBBY_ADMIN_EMAILS", "ryan.c.kohler@gmail.com")

	resolver := &Resolver{}
	email := "ryan.c.kohler@gmail.com"
	userResolver := resolver.User()

	isAdmin, err := userResolver.IsAdmin(context.Background(), &model.User{Email: &email})
	if err != nil {
		t.Fatalf("IsAdmin failed: %v", err)
	}
	if !isAdmin {
		t.Fatal("expected allowlisted email to resolve isAdmin=true")
	}

	other := "player@example.com"
	isAdmin, err = userResolver.IsAdmin(context.Background(), &model.User{Email: &other})
	if err != nil {
		t.Fatalf("IsAdmin failed: %v", err)
	}
	if isAdmin {
		t.Fatal("expected non-allowlisted email to resolve isAdmin=false")
	}
}
