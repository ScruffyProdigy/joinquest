package graph

import (
	"errors"
	"testing"

	"github.com/scruffyprodigy/joinquest/internal/pubsub"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestResolvedAPIBaseURL(t *testing.T) {
	t.Setenv("GAME_API_BASE_URL", "http://game-api.test")

	r := NewResolver(nil, nil, pubsub.NewMemory())

	game := &store.Game{Name: "Test"}
	if got := r.resolvedAPIBaseURL(game); got != "http://game-api.test" {
		t.Fatalf("resolvedAPIBaseURL = %q, want http://game-api.test", got)
	}

	api := "http://catalog-api.test"
	game.APIBaseURL = &api
	if got := r.resolvedAPIBaseURL(game); got != "http://catalog-api.test" {
		t.Fatalf("resolvedAPIBaseURL = %q, want catalog override", got)
	}
}

func TestProvisionPlayerFromUserRequiresAName(t *testing.T) {
	name := "Ada"
	avatar := "http://localhost:5173/avatars/storm.png"

	t.Run("carries the chosen name and face", func(t *testing.T) {
		player, err := provisionPlayerFromUser(&store.User{DisplayName: &name, AvatarURL: &avatar})
		if err != nil {
			t.Fatalf("provisionPlayerFromUser: %v", err)
		}
		if player.DisplayName != "Ada" {
			t.Fatalf("displayName = %q, want Ada", player.DisplayName)
		}
		if player.AvatarURL != avatar {
			t.Fatalf("avatarUrl = %q, want %q", player.AvatarURL, avatar)
		}
	})

	t.Run("a face without a name is not enough", func(t *testing.T) {
		// The old behaviour returned a player block here with an empty name,
		// and no block at all when the avatar was missing too.
		if _, err := provisionPlayerFromUser(&store.User{AvatarURL: &avatar}); !errors.Is(err, ErrPlayerIdentityMissing) {
			t.Fatalf("want ErrPlayerIdentityMissing, got %v", err)
		}
	})

	t.Run("a blank name is no name", func(t *testing.T) {
		blank := "   "
		if _, err := provisionPlayerFromUser(&store.User{DisplayName: &blank}); !errors.Is(err, ErrPlayerIdentityMissing) {
			t.Fatalf("want ErrPlayerIdentityMissing, got %v", err)
		}
	})

	t.Run("no user at all", func(t *testing.T) {
		if _, err := provisionPlayerFromUser(nil); !errors.Is(err, ErrPlayerIdentityMissing) {
			t.Fatalf("want ErrPlayerIdentityMissing, got %v", err)
		}
	})
}
