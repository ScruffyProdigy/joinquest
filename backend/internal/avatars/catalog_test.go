package avatars

import "testing"

func TestStarterByKey(t *testing.T) {
	entry, ok := StarterByKey("Compass")
	if !ok || entry.Key != "compass" {
		t.Fatalf("expected compass, got %+v ok=%v", entry, ok)
	}
	_, ok = StarterByKey("unknown")
	if ok {
		t.Fatal("expected unknown key to miss")
	}
}

func TestPublicAssetURL(t *testing.T) {
	got := PublicAssetURL("https://joinquest.cc", "compass.png")
	want := "https://joinquest.cc/avatars/compass.png"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveURLPrefersStored(t *testing.T) {
	stored := "https://cdn.example/avatar.png"
	got := ResolveURL("https://joinquest.cc", &stored, ptr("compass"))
	if got == nil || *got != stored {
		t.Fatalf("expected stored url, got %+v", got)
	}
}

func TestResolveURLFromKey(t *testing.T) {
	key := "beacon"
	got := ResolveURL("https://joinquest.cc", nil, &key)
	if got == nil || *got != "https://joinquest.cc/avatars/beacon.png" {
		t.Fatalf("unexpected url: %+v", got)
	}
}

func TestResolveURLAbsolutizesSpiritAvatarPath(t *testing.T) {
	stored := "/spirit-avatars/reading-id/ember-fox.png"
	got := ResolveURL("https://joinquest.cc", &stored, nil)
	if got == nil || *got != "https://joinquest.cc/spirit-avatars/reading-id/ember-fox.png" {
		t.Fatalf("unexpected url: %+v", got)
	}
}

func TestAbsolutizePublicAssetURL(t *testing.T) {
	if got := AbsolutizePublicAssetURL("https://joinquest.cc", "/avatars/storm.png"); got != "https://joinquest.cc/avatars/storm.png" {
		t.Fatalf("relative: got %q", got)
	}
	if got := AbsolutizePublicAssetURL("https://joinquest.cc", "https://cdn.example/a.png"); got != "https://cdn.example/a.png" {
		t.Fatalf("absolute: got %q", got)
	}
}

func ptr(s string) *string { return &s }

func TestSigilByKeyResolvesEveryFamilyAndTint(t *testing.T) {
	for _, family := range SigilFamilies {
		for _, tint := range SigilTints {
			key := "sigil-" + family + "-" + tint
			entry, ok := SigilByKey(key)
			if !ok {
				t.Fatalf("SigilByKey(%q) not found", key)
			}
			want := "sigils/" + family + "-" + tint + ".svg"
			if entry.File != want {
				t.Fatalf("SigilByKey(%q) file = %q, want %q", key, entry.File, want)
			}
			if _, clash := StarterByKey(key); clash {
				t.Fatalf("sigil key %q collides with a starter avatar key", key)
			}
		}
	}
}

func TestSigilByKeyRejectsUnknown(t *testing.T) {
	for _, key := range []string{
		"compass",                 // starter avatar, not a sigil
		"",                        // empty
		"sigil-canine",            // missing tint
		"sigil-canine-chartreuse", // unknown tint
		"sigil-dragon-frost",      // unknown family
		"canine-frost",            // missing prefix
		"sigil-canine-frost-extra",
	} {
		if _, ok := SigilByKey(key); ok {
			t.Fatalf("SigilByKey(%q) should not resolve", key)
		}
	}
}
