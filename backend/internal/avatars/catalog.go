package avatars

import (
	"fmt"
	"strings"
)

const SourceStarter = "starter"

// StarterEntry is a pickable avatar from the static catalog.
type StarterEntry struct {
	Key  string
	Name string
	Slot string
	File string
}

// StarterCatalog is the default avatar set for profile pickers (18 options).
var StarterCatalog = []StarterEntry{
	{Key: "compass", Name: "Compass", Slot: "Compass", File: "compass.png"},
	{Key: "coin", Name: "Coin", Slot: "Coin", File: "coin.png"},
	{Key: "storm", Name: "Storm", Slot: "Storm", File: "storm.png"},
	{Key: "campfire", Name: "Campfire", Slot: "Campfire", File: "campfire.png"},
	{Key: "beacon", Name: "Beacon", Slot: "Beacon", File: "beacon.png"},
	{Key: "angel", Name: "Angel", Slot: "Angel", File: "angel.png"},
	{Key: "bell", Name: "Bell", Slot: "Bell", File: "bell.png"},
	{Key: "constellation", Name: "Constellation", Slot: "Constellation", File: "constellation.png"},
	{Key: "crown", Name: "Crown", Slot: "Crown", File: "crown.png"},
	{Key: "footsteps", Name: "Footsteps", Slot: "Footsteps", File: "footsteps.png"},
	{Key: "forge", Name: "Forge", Slot: "Forge", File: "forge.png"},
	{Key: "goblet", Name: "Goblet", Slot: "Goblet", File: "goblet.png"},
	{Key: "jester", Name: "Jester", Slot: "Jester", File: "jester.png"},
	{Key: "key", Name: "Key", Slot: "Key", File: "key.png"},
	{Key: "kite", Name: "Kite", Slot: "Kite", File: "kite.png"},
	{Key: "moon", Name: "Moon", Slot: "Moon", File: "moon.png"},
	{Key: "ring", Name: "Ring", Slot: "Ring", File: "ring.png"},
	{Key: "rope", Name: "Rope", Slot: "Rope", File: "rope.png"},
}

// StarterByKey returns a catalog entry by key (case-insensitive).
func StarterByKey(key string) (StarterEntry, bool) {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, entry := range StarterCatalog {
		if entry.Key == normalized {
			return entry, true
		}
	}
	return StarterEntry{}, false
}

// PublicAssetURL builds an absolute HTTPS URL for a starter avatar asset.
func PublicAssetURL(publicOrigin, file string) string {
	base := strings.TrimRight(strings.TrimSpace(publicOrigin), "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	return fmt.Sprintf("%s/avatars/%s", base, strings.TrimPrefix(file, "/"))
}

// AbsolutizePublicAssetURL turns a stored relative avatar path into an absolute URL
// for cross-origin game clients. Absolute URLs are returned unchanged.
func AbsolutizePublicAssetURL(publicOrigin, url string) string {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "/") {
		base := strings.TrimRight(strings.TrimSpace(publicOrigin), "/")
		if base == "" {
			base = "http://localhost:5173"
		}
		return base + trimmed
	}
	return trimmed
}

// ResolveURL returns the user's public avatar URL, preferring a stored value.
func ResolveURL(publicOrigin string, storedURL, avatarKey *string) *string {
	if storedURL != nil {
		trimmed := strings.TrimSpace(*storedURL)
		if trimmed != "" {
			out := AbsolutizePublicAssetURL(publicOrigin, trimmed)
			return &out
		}
	}
	if avatarKey == nil {
		return nil
	}
	entry, ok := StarterByKey(*avatarKey)
	if !ok {
		return nil
	}
	url := PublicAssetURL(publicOrigin, entry.File)
	return &url
}
