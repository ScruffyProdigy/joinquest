package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/lfg/partytree"
)

// Legacy parties stored seat key as tree role (pre-BuildFromPinnedSeats fix).
func TestReconcileFormingHealsLegacyFlatDuelParties(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	queueID := DemoDefaultQueueID
	userA, err := st.CreateUser(ctx, CreateUserParams{Email: "legacy-duel-a-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	cleaner.TrackUser(userA.ID)
	userB, err := st.CreateUser(ctx, CreateUserParams{Email: "legacy-duel-b-" + uuid.NewString() + "@example.com"})
	if err != nil {
		t.Fatalf("CreateUser B: %v", err)
	}
	cleaner.TrackUser(userB.ID)

	legacyTree := func(userID string) json.RawMessage {
		tree := partytree.Node{Children: []partytree.Node{{Role: "1", Members: []string{userID}}}}
		raw, err := json.Marshal(tree)
		if err != nil {
			t.Fatalf("marshal tree: %v", err)
		}
		return raw
	}

	var partyAID uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		INSERT INTO parties (leader_user_id, mode_queue_id, status, party_tree)
		VALUES ($1, $2, 'waiting', $3)
		RETURNING id
	`, userA.ID, queueID, legacyTree(userA.ID.String())).Scan(&partyAID); err != nil {
		t.Fatalf("insert party A: %v", err)
	}
	var partyBID uuid.UUID
	if err := st.db.QueryRowContext(ctx, `
		INSERT INTO parties (leader_user_id, mode_queue_id, status, party_tree)
		VALUES ($1, $2, 'waiting', $3)
		RETURNING id
	`, userB.ID, queueID, legacyTree(userB.ID.String())).Scan(&partyBID); err != nil {
		t.Fatalf("insert party B: %v", err)
	}

	for _, row := range []struct {
		userID, partyID uuid.UUID
	}{
		{userA.ID, partyAID},
		{userB.ID, partyBID},
	} {
		if _, err := st.db.ExecContext(ctx, `
			INSERT INTO party_members (party_id, user_id, queue_path, sort_order)
			VALUES ($1, $2, '', 0)
		`, row.partyID, row.userID); err != nil {
			t.Fatalf("insert member: %v", err)
		}
		if _, err := st.db.ExecContext(ctx, `
			INSERT INTO game_queues (game_id, user_id, status, mode_queue_id, queue_path, party_id)
			VALUES ($1, $2, 'waiting', $3, '', $4)
		`, DemoPrimaryGameID, row.userID, queueID, row.partyID); err != nil {
			t.Fatalf("insert queue row: %v", err)
		}
	}

	rec := mustReconcileForming(t, st, ctx, queueID)
	if !rec.Fired || rec.SessionID == nil {
		t.Fatalf("expected legacy flat duel parties to match, got %+v", rec)
	}

	participants, err := st.ListSessionSeatAssignments(ctx, *rec.SessionID)
	if err != nil {
		t.Fatalf("ListSessionSeatAssignments: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
}
