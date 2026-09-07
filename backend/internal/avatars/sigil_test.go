package avatars

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSigilByKeyResolvesEveryFamily(t *testing.T) {
	for _, family := range SigilFamilies {
		key := "sigil-" + family + "-3b82f6"
		entry, ok := SigilByKey(key)
		if !ok {
			t.Fatalf("SigilByKey(%q) not found", key)
		}
		want := "sigils/" + family + "-3b82f6.svg"
		if entry.File != want {
			t.Fatalf("SigilByKey(%q) file = %q, want %q", key, entry.File, want)
		}
		if _, clash := StarterByKey(key); clash {
			t.Fatalf("sigil key %q collides with a starter avatar key", key)
		}
	}
}

// Guests who picked an avatar before the picker drew its own colours still hold
// keys naming one of the twelve old tints, so those must never stop resolving.
func TestSigilByKeyResolvesLegacyTintNames(t *testing.T) {
	for tint, hex := range legacySigilTints {
		key := "sigil-canine-" + tint
		entry, ok := SigilByKey(key)
		if !ok {
			t.Fatalf("SigilByKey(%q) not found", key)
		}
		if want := "sigils/canine-" + hex + ".svg"; entry.File != want {
			t.Fatalf("SigilByKey(%q) file = %q, want %q", key, entry.File, want)
		}
	}
}

func TestSigilByKeyRejectsUnknown(t *testing.T) {
	for _, key := range []string{
		"compass",                 // starter avatar, not a sigil
		"",                        // empty
		"sigil-canine",            // missing colour
		"sigil-canine-chartreuse", // neither a hex nor a legacy tint
		"sigil-canine-3b82f",      // five hex digits
		"sigil-canine-3b82f6a",    // seven hex digits
		"sigil-canine-#3b82f6",    // hex must be bare
		"sigil-dragon-3b82f6",     // unknown family
		"canine-3b82f6",           // missing prefix
		"sigil-canine-3b82f6-x",
	} {
		if _, ok := SigilByKey(key); ok {
			t.Fatalf("SigilByKey(%q) should not resolve", key)
		}
	}
}

func TestRenderSigilDrawsEveryFamilyInItsColour(t *testing.T) {
	for _, family := range SigilFamilies {
		svg, ok := RenderSigil(family, "3b82f6")
		if !ok {
			t.Fatalf("RenderSigil(%q) not drawn", family)
		}
		if !strings.Contains(svg, `<circle cx="32" cy="32" r="32" fill="#3b82f6"/>`) {
			t.Fatalf("RenderSigil(%q) is missing its tinted disc: %s", family, svg)
		}
		mark := sigilMark("3b82f6")
		if !strings.Contains(svg, `fill="`+mark+`"`) {
			t.Fatalf("RenderSigil(%q) draws no silhouette: %s", family, svg)
		}
		if !strings.Contains(svg, `stroke="`+mark+`"`) {
			t.Fatalf("RenderSigil(%q) draws no rim: %s", family, svg)
		}
	}
}

func TestSigilHandlerServesRenderedSVG(t *testing.T) {
	rec := httptest.NewRecorder()
	SigilHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/avatars/sigils/canine-3b82f6.svg", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("Content-Type = %q, want image/svg+xml", got)
	}
	if !strings.Contains(rec.Body.String(), "#3b82f6") {
		t.Fatalf("body is not drawn in the requested colour: %s", rec.Body.String())
	}
}

// A light disc has to flip to the dark mark, or the silhouette washes out on it.
func TestRenderSigilFlipsTheMarkOnLightDiscs(t *testing.T) {
	dark, _ := RenderSigil("canine", "18324a")
	if !strings.Contains(dark, sigilPale) || strings.Contains(dark, sigilInk) {
		t.Fatalf("a dark disc should carry the pale mark: %s", dark)
	}
	light, _ := RenderSigil("canine", "cfe6f5")
	if !strings.Contains(light, sigilInk) || strings.Contains(light, sigilPale) {
		t.Fatalf("a light disc should carry the dark mark: %s", light)
	}
}

func TestSigilHandlerRejectsBadPaths(t *testing.T) {
	for _, path := range []string{
		"/avatars/sigils/canine-3b82f6",      // no extension
		"/avatars/sigils/canine-notahex.svg", // unknown colour
		"/avatars/sigils/dragon-3b82f6.svg",  // unknown family
		"/avatars/sigils/canine.svg",         // no colour
		"/avatars/sigils/",
	} {
		rec := httptest.NewRecorder()
		SigilHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}
