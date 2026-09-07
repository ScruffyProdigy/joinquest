package graph

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestToGraphQLUser(t *testing.T) {
	email := "user@example.com"
	displayName := "Test User"
	createdAt := time.Now().UTC().Truncate(time.Second)

	user := ToGraphQLUser(&store.User{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		Email:       email,
		Username:    "user",
		DisplayName: &displayName,
		CreatedAt:   createdAt,
	})

	if user.Email == nil || *user.Email != email {
		t.Fatalf("unexpected email: %+v", user.Email)
	}
	if user.DisplayName == nil || *user.DisplayName != displayName {
		t.Fatalf("unexpected display name: %+v", user.DisplayName)
	}

	// A player who never picked one maps through as absent, not as a placeholder.
	nameless := ToGraphQLUser(&store.User{ID: uuid.New(), Username: "guest_x", IsGuest: true})
	if nameless.DisplayName != nil {
		t.Fatalf("expected no display name, got %q", *nameless.DisplayName)
	}
}

func TestToGraphQLSessionStatus(t *testing.T) {
	cases := map[string]model.SessionStatus{
		"active":    model.SessionStatusActive,
		"completed": model.SessionStatusEnded,
		"cancelled": model.SessionStatusEnded,
		"unknown":   model.SessionStatusPending,
	}
	for input, want := range cases {
		if got := ToGraphQLSessionStatus(input); got != want {
			t.Fatalf("ToGraphQLSessionStatus(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToGraphQLDigitalGoodUsesIDAsCode(t *testing.T) {
	id := uuid.New()
	good := ToGraphQLDigitalGood(&store.DigitalGood{
		ID:   id,
		Name: "Skin",
	})
	if good.Code != id.String() {
		t.Fatalf("expected code %q, got %q", id.String(), good.Code)
	}
}
