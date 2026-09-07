package avatars

import (
	"math"
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
		if !strings.Contains(svg, `<circle cx="32" cy="32" r="32" fill="url(#g`+family+`3b82f6)"/>`) {
			t.Fatalf("RenderSigil(%q) is missing its tinted disc: %s", family, svg)
		}
		mark := sigilMark("3b82f6")
		if !strings.Contains(svg, `fill="`+mark+`"`) {
			t.Fatalf("RenderSigil(%q) draws no silhouette: %s", family, svg)
		}
		if !strings.Contains(svg, `stroke="`+mark+`"`) {
			t.Fatalf("RenderSigil(%q) draws no rim: %s", family, svg)
		}
		if !strings.Contains(svg, "<linearGradient") {
			t.Fatalf("RenderSigil(%q) draws no gradient: %s", family, svg)
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
	// The base colour no longer appears literally: the disc is filled by a gradient
	// whose two stops sit either side of that hue.
	if !strings.Contains(rec.Body.String(), "url(#gcanine3b82f6)") {
		t.Fatalf("body is not drawn in the requested colour: %s", rec.Body.String())
	}
	from, to := sigilGradient("3b82f6")
	for _, stop := range []string{from, to} {
		if !strings.Contains(rec.Body.String(), stop) {
			t.Fatalf("body is missing gradient stop %s: %s", stop, rec.Body.String())
		}
	}
}

// The mark carries the whole legibility guarantee, so it is checked across the
// lightness range the picker actually draws from rather than at a single colour.
func TestSigilMarkMeetsItsContrastEverywhere(t *testing.T) {
	for _, hex := range []string{
		"0a2312", "123a5c", "2f6f3f", "7a3f9c", "c04a2a",
		"9ad4a8", "cfe6f5", "f0c8d8", "e8e2b0", "f5f7f2",
	} {
		disc := luminance(parseHex(hex))
		mark := luminance(parseHex(strings.TrimPrefix(sigilMark(hex), "#")))
		got := (math.Max(disc, mark) + 0.05) / (math.Min(disc, mark) + 0.05)
		if math.Abs(got-MarkContrast) > 0.06 {
			t.Fatalf("sigilMark(%q) contrast = %.2f:1, want %.2f:1", hex, got, MarkContrast)
		}
	}
}

// A mark that washed out to white or black would defeat the point of deriving it
// from the disc, so it has to keep some of the disc's own colour.
func TestSigilMarkKeepsTheDiscsHue(t *testing.T) {
	for _, hex := range []string{"0a2312", "2f6f3f", "c04a2a", "9ad4a8", "f0c8d8"} {
		discHue, _, _ := rgbToHSL(parseHex(hex))
		markHue, markSat, _ := rgbToHSL(parseHex(strings.TrimPrefix(sigilMark(hex), "#")))
		drift := math.Abs(markHue - discHue)
		if drift > 180 {
			drift = 360 - drift
		}
		if drift > 5 {
			t.Fatalf("sigilMark(%q) drifted %.1f degrees off the disc hue", hex, drift)
		}
		if markSat < 20 {
			t.Fatalf("sigilMark(%q) washed out to %.0f%% saturation", hex, markSat)
		}
	}
}

// The gradient is a hue rotation, not a lightness ramp: both stops sit on the
// disc's own luminance so the sweep spends none of the contrast budget.
func TestSigilGradientHoldsLuminance(t *testing.T) {
	for _, hex := range []string{"0a2312", "123a5c", "c04a2a", "9ad4a8", "e8e2b0"} {
		want := luminance(parseHex(hex))
		from, to := sigilGradient(hex)
		for _, stop := range []string{from, to} {
			got := luminance(parseHex(strings.TrimPrefix(stop, "#")))
			if math.Abs(got-want) > 0.005 {
				t.Fatalf("gradient stop %s of %q is Y %.4f, want %.4f", stop, hex, got, want)
			}
		}
		if from == to {
			t.Fatalf("gradient for %q does not sweep", hex)
		}
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
