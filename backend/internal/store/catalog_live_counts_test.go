package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCountLivePlayersByGame(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	game, err := st.InsertTestGame(ctx, "Live Counts Game")
	if err != nil {
		t.Fatalf("InsertTestGame: %v", err)
	}
	cleaner.TrackGame(game.ID)

	quiet, err := st.InsertTestGame(ctx, "Quiet Game")
	if err != nil {
		t.Fatalf("InsertTestGame quiet: %v", err)
	}
	cleaner.TrackGame(quiet.ID)

	newUser := func(label string) uuid.UUID {
		t.Helper()
		user, err := st.CreateUser(ctx, CreateUserParams{
			Email:       label + "-" + uuid.NewString() + "@example.com",
			DisplayName: label,
		})
		if err != nil {
			t.Fatalf("CreateUser %s: %v", label, err)
		}
		cleaner.TrackUser(user.ID)
		return user.ID
	}

	seated := newUser("seated")
	departed := newUser("departed")
	waiting := newUser("waiting")
	stale := newUser("stale")

	session, err := st.CreateSession(ctx, game.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	for _, userID := range []uuid.UUID{seated, departed} {
		if err := st.AddSessionParticipant(ctx, session.ID, userID, "player"); err != nil {
			t.Fatalf("AddSessionParticipant: %v", err)
		}
	}
	// A player who already went back to the lobby is no longer playing.
	if err := st.MarkParticipantFinished(ctx, session.ID, departed, time.Now()); err != nil {
		t.Fatalf("MarkParticipantFinished: %v", err)
	}

	if _, err := st.db.ExecContext(ctx, `
		INSERT INTO game_queues (game_id, user_id, status)
		VALUES ($1, $2, 'waiting')
	`, game.ID, waiting); err != nil {
		t.Fatalf("insert waiting row: %v", err)
	}

	// A session the game server never closed out must not keep inflating the card forever.
	var staleSessionID uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		INSERT INTO game_sessions (game_id, status, started_at)
		VALUES ($1, 'active', $2)
		RETURNING id
	`, game.ID, time.Now().Add(-stalePlayingMaxAge()-time.Hour)).Scan(&staleSessionID); err != nil {
		t.Fatalf("insert stale session: %v", err)
	}
	if err := st.AddSessionParticipant(ctx, staleSessionID, stale, "player"); err != nil {
		t.Fatalf("AddSessionParticipant stale: %v", err)
	}

	counts, err := st.CountLivePlayersByGame(ctx)
	if err != nil {
		t.Fatalf("CountLivePlayersByGame: %v", err)
	}

	got := counts[game.ID]
	if got.Playing != 1 {
		t.Fatalf("playing = %d, want 1 (seated only)", got.Playing)
	}
	if got.Queued != 1 {
		t.Fatalf("queued = %d, want 1", got.Queued)
	}

	if _, ok := counts[quiet.ID]; ok {
		t.Fatalf("a game with no activity should be absent from the map, got %+v", counts[quiet.ID])
	}
}
