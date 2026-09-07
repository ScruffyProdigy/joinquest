package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/testdb"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()

	databaseURL := testdb.RequireURL(t)

	db, err := openDB(databaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return New(db)
}

func TestStoreUserGameFlow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	email := "store-test-" + uuid.NewString() + "@example.com"
	user, err := st.CreateUser(ctx, CreateUserParams{
		Email:       email,
		DisplayName: "Store Test User",
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	cleaner.TrackUser(user.ID)

	found, err := st.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if found.ID != user.ID {
		t.Fatalf("expected user id %s, got %s", user.ID, found.ID)
	}

	games, err := st.ListCatalogGames(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListCatalogGames failed: %v", err)
	}
	if len(games) == 0 {
		t.Fatal("expected at least one catalog game")
	}

	result, err := st.JoinModeQueue(ctx, DemoDefaultQueueID, user.ID, "", nil)
	if err != nil {
		t.Fatalf("JoinModeQueue failed: %v", err)
	}
	if result.Status != QueueStatusWaiting {
		t.Fatalf("expected waiting queue status, got %s", result.Status)
	}

	if _, err := st.LeaveModeQueue(ctx, DemoDefaultQueueID, user.ID); err != nil {
		t.Fatalf("LeaveModeQueue failed: %v", err)
	}
}

func TestStoreMagicLinkConsume(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	email := "magic-" + uuid.NewString() + "@example.com"
	cleaner.TrackEmail(email)

	token := uuid.NewString()
	tokenHash := "test-hash-" + uuid.NewString()
	link, err := st.CreateMagicLink(ctx, CreateMagicLinkParams{
		Email:     email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateMagicLink failed: %v", err)
	}
	_ = token

	if !st.IsMagicLinkValid(link, time.Now()) {
		t.Fatal("expected magic link to be valid")
	}

	consumed, err := st.ConsumeMagicLinkByTokenHash(ctx, tokenHash, time.Now())
	if err != nil {
		t.Fatalf("ConsumeMagicLinkByTokenHash failed: %v", err)
	}
	if consumed.ID != link.ID {
		t.Fatalf("expected link id %s, got %s", link.ID, consumed.ID)
	}

	_, err = st.ConsumeMagicLinkByTokenHash(ctx, tokenHash, time.Now())
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on reuse, got %v", err)
	}
}

func TestStoreInventoryFlow(t *testing.T) {
	st := openTestStore(t)
	cleaner := st.NewTestCleaner(t)
	ctx := context.Background()

	user, err := st.CreateUser(ctx, CreateUserParams{
		Email:       "inventory-" + uuid.NewString() + "@example.com",
		DisplayName: "Inventory User",
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	cleaner.TrackUser(user.ID)

	game, err := st.InsertTestGame(ctx, "Inventory Game")
	if err != nil {
		t.Fatalf("InsertTestGame failed: %v", err)
	}
	cleaner.TrackGame(game.ID)

	description := "A test item"
	good, err := st.CreateDigitalGood(ctx, "Test Good", &description, &game.ID)
	if err != nil {
		t.Fatalf("CreateDigitalGood failed: %v", err)
	}

	if err := st.GrantInventoryItem(ctx, user.ID, good.ID, 2); err != nil {
		t.Fatalf("GrantInventoryItem failed: %v", err)
	}

	items, err := st.ListUserInventory(ctx, user.ID, &game.ID)
	if err != nil {
		t.Fatalf("ListUserInventory failed: %v", err)
	}
	if len(items) != 1 || items[0].Quantity != 2 {
		t.Fatalf("unexpected inventory: %+v", items)
	}

	if err := st.RevokeInventoryItem(ctx, user.ID, good.ID, 1); err != nil {
		t.Fatalf("RevokeInventoryItem failed: %v", err)
	}

	items, err = st.ListUserInventory(ctx, user.ID, &game.ID)
	if err != nil {
		t.Fatalf("ListUserInventory failed: %v", err)
	}
	if len(items) != 1 || items[0].Quantity != 1 {
		t.Fatalf("expected quantity 1 after revoke, got %+v", items)
	}
}
