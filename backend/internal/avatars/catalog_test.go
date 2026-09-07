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
