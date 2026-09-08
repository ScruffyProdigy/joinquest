package store

import (
	"context"
	"testing"

	"github.com/lib/pq"
)

// JQ-203: production showed two "Rock Paper Scissors Lizard Robot" cards because
// the k8s handoff job reassigned slugs after the migrations had written identity
// onto the seed ids, so the row at slug word-hunt wore RPSLR's whole identity.
// These tests read the migrated database, which is where that disagreement
// became visible. The other half of the guard -- that no writer outside
// migration 000050 sets identity, and that each card's handoff host matches its
// slug -- is static and lives in scripts/check-catalog-identity.mjs, because
// integration tests repoint api_base_url at localhost and so cannot assert it.

func TestCatalogGameNamesAreUnique(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	// Scoped to the seeded a1000000-… block. Integration tests create catalog
	// games with generated slugs and a shared mock name in this database, so an
	// unscoped query would report those instead of a real seed collision.
	rows, err := st.db.QueryContext(ctx, `
		SELECT lower(name), array_agg(slug ORDER BY slug)
		FROM games
		WHERE id::text LIKE 'a1000000-0000-4000-8000-%'
		  AND status = 'active'
		  AND category = 'catalog'
		GROUP BY lower(name)
		HAVING count(*) > 1
	`)
	if err != nil {
		t.Fatalf("query duplicate names: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var slugs pq.StringArray
		if err := rows.Scan(&name, &slugs); err != nil {
			t.Fatalf("scan: %v", err)
		}
		t.Errorf("catalog name %q is shared by slugs %v — two cards advertise the same game", name, []string(slugs))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
}

func TestSeededCatalogGamesKeepTheirOwnIdentity(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	want := map[string]struct {
		name     string
		iconURL  string
		heroURL  string
		tags     []string
		tutorial string
	}{
		"word-hunt": {
			name:     "Word Hunt",
			iconURL:  "/games/word-hunt-icon.png",
			heroURL:  "/games/word-hunt-hero.jpg",
			tags:     []string{"party", "competitive", "words"},
			tutorial: "https://word-hunt-arena.win",
		},
		"rock-paper-scissors-lizard-robot": {
			name:     "Rock Paper Scissors Lizard Robot",
			iconURL:  "/games/rpslr-icon.png",
			heroURL:  "/games/rpslr-hero.jpg",
			tags:     []string{"competitive", "1v1", "quick"},
			tutorial: "https://rpsls-duel.win",
		},
	}

	for slug, expected := range want {
		var name, iconURL, heroURL, tutorial string
		var shortDescription string
		var tags pq.StringArray
		err := st.db.QueryRowContext(ctx, `
			SELECT name, icon_url, hero_url, COALESCE(tutorial_url, ''), COALESCE(short_description, ''), tags
			FROM games
			WHERE slug = $1
		`, slug).Scan(&name, &iconURL, &heroURL, &tutorial, &shortDescription, &tags)
		if err != nil {
			t.Errorf("slug %s: %v", slug, err)
			continue
		}

		if name != expected.name {
			t.Errorf("slug %s: name = %q, want %q", slug, name, expected.name)
		}
		if iconURL != expected.iconURL {
			t.Errorf("slug %s: icon_url = %q, want %q", slug, iconURL, expected.iconURL)
		}
		if heroURL != expected.heroURL {
			t.Errorf("slug %s: hero_url = %q, want %q", slug, heroURL, expected.heroURL)
		}
		if tutorial != expected.tutorial {
			t.Errorf("slug %s: tutorial_url = %q, want %q", slug, tutorial, expected.tutorial)
		}
		if shortDescription == "" {
			t.Errorf("slug %s: short_description is empty — the catalog card has no blurb", slug)
		}
		if got := []string(tags); !equalStrings(got, expected.tags) {
			t.Errorf("slug %s: tags = %v, want %v", slug, got, expected.tags)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
