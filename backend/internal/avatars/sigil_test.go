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
		want := "sigils/" + family + "-3b82f6-wide.svg"
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
		if want := "sigils/canine-" + hex + "-wide.svg"; entry.File != want {
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
		svg, ok := RenderSigil(family, "3b82f6", "wide")
		if !ok {
			t.Fatalf("RenderSigil(%q) not drawn", family)
		}
		if !strings.Contains(svg, `<circle cx="32" cy="32" r="32" fill="url(#g`+family+`3b82f6wide)"/>`) {
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
	SigilHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/avatars/sigils/canine-3b82f6-wide.svg", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("Content-Type = %q, want image/svg+xml", got)
	}
	// The base colour no longer appears literally: the disc is filled by a gradient
	// whose two stops sit either side of that hue.
	if !strings.Contains(rec.Body.String(), "url(#gcanine3b82f6wide)") {
		t.Fatalf("body is not drawn in the requested colour: %s", rec.Body.String())
	}
	from, to := sigilGradient("3b82f6")
	for _, stop := range []string{from, to} {
		if !strings.Contains(rec.Body.String(), stop) {
			t.Fatalf("body is missing gradient stop %s: %s", stop, rec.Body.String())
		}
	}
}

// The mark carries the whole legibility guarantee, and it has to hold across the
// disc's sweep rather than only at its middle — so both gradient stops are checked,
// across the lightness range the picker actually draws from.
func TestSigilMarkMeetsItsContrastAcrossTheSweep(t *testing.T) {
	for _, hex := range []string{
		"0a2312", "123a5c", "2f6f3f", "7a3f9c", "c04a2a", "8a8420",
		"9ad4a8", "cfe6f5", "f0c8d8", "e8e2b0", "f5f7f2", "4a4a4a",
	} {
		mark := luminance(parseHex(strings.TrimPrefix(sigilMark(hex), "#")))
		from, to := sigilGradient(hex)
		for _, stop := range []string{from, to, "#" + hex} {
			disc := luminance(parseHex(strings.TrimPrefix(stop, "#")))
			got := (math.Max(disc, mark) + 0.05) / (math.Min(disc, mark) + 0.05)
			if got < MarkContrast-0.05 {
				t.Fatalf("mark on %q reads %.2f:1 against %s, want at least %.2f:1",
					hex, got, stop, MarkContrast)
			}
		}
	}
}

// A mark that washed out to white or black would defeat the point of deriving it
// from the disc. This is the check that catches an unreachable contrast target:
// when the solve cannot land, it bottoms out at black and the hue goes with it.
func TestSigilMarkKeepsTheDiscsHue(t *testing.T) {
	for _, hex := range []string{
		"0a2312", "2f6f3f", "c04a2a", "9ad4a8", "f0c8d8", "8a8420", "123a5c",
	} {
		discHue, _, _ := rgbToHSL(parseHex(hex))
		markRGB := parseHex(strings.TrimPrefix(sigilMark(hex), "#"))
		markHue, markSat, _ := rgbToHSL(markRGB)
		if drift := hueGap(markHue, discHue); drift > 5 {
			t.Fatalf("sigilMark(%q) drifted %.1f degrees off the disc hue", hex, drift)
		}
		if markSat < 20 {
			t.Fatalf("sigilMark(%q) washed out to %.0f%% saturation", hex, markSat)
		}
	}
}

// The sweep is specified in perceived degrees so that every disc gets the same
// visible gradient. Measured in HSL degrees instead it ran ten times stronger in
// cyan than in green.
func TestSigilGradientSweepsEvenlyByEye(t *testing.T) {
	var spans []float64
	for _, hex := range []string{
		"b32d1e", "b3721e", "8a8420", "3f9a2c", "1f9a6a", "1f8f9a",
		"2a5fb3", "5a3fb3", "9a2f8a", "b32d5e",
	} {
		from, to := sigilGradient(hex)
		span := hueGap(
			perceivedHue(parseHex(strings.TrimPrefix(from, "#"))),
			perceivedHue(parseHex(strings.TrimPrefix(to, "#"))),
		)
		if span < gradientSweep {
			t.Fatalf("gradient on %q sweeps only %.1f perceived degrees", hex, span)
		}
		spans = append(spans, span)
	}
	low, high := spans[0], spans[0]
	for _, v := range spans {
		low, high = math.Min(low, v), math.Max(high, v)
	}
	if high/low > 1.35 {
		t.Fatalf("sweep is uneven by eye: %.1f to %.1f perceived degrees", low, high)
	}
}

// The two stops part in lightness as well as hue, always the same way round so a
// row of avatars reads as lit from one place. The light stop is bounded by the
// headroom left below markFlip and the dark one is not, so a disc near the ceiling
// still gets a gradient instead of almost none.
func TestSigilGradientLiftUsesTheHeadroomItHas(t *testing.T) {
	spread := func(hex string) (float64, float64) {
		from, to := sigilGradient(hex)
		a := lightnessStar(luminance(parseHex(strings.TrimPrefix(from, "#"))))
		b := lightnessStar(luminance(parseHex(strings.TrimPrefix(to, "#"))))
		return a, b
	}
	for _, hex := range []string{"0a2312", "123a5c", "2f6f3f", "c04a2a", "1f8f9a"} {
		baseL := lightnessStar(luminance(parseHex(hex)))
		up, down := gradientLifts(baseL)
		light, dark := spread(hex)
		if light <= dark {
			t.Fatalf("gradient on %q does not lift its first stop: L* %.1f then %.1f", hex, light, dark)
		}
		if want := up + down; math.Abs((light-dark)-want) > 1.5 {
			t.Fatalf("gradient on %q spreads %.1f L*, want %.1f", hex, light-dark, want)
		}
		// A fixture may already sit above the ceiling; what matters is that the
		// lift never pushes it further up.
		if light > math.Max(baseL, liftCeiling)+0.5 {
			t.Fatalf("gradient on %q lifts to L* %.1f, past the ceiling at %.1f", hex, light, liftCeiling)
		}
	}

	// Every disc gets a real gradient, including one sitting at the ceiling.
	for _, baseL := range []float64{30, 36, 42, 46, 49} {
		up, down := gradientLifts(baseL)
		if up+down < gradientLift/2 {
			t.Fatalf("a disc at L* %.0f only spreads %.1f L*", baseL, up+down)
		}
	}
}

// An expression is optional in a key. Everything saved before expressions existed
// has two parts, and has to keep resolving to the plain face it already renders.
func TestSigilByKeyDefaultsTheExpression(t *testing.T) {
	for _, key := range []string{"sigil-canine-3b82f6", "sigil-canine-frost"} {
		entry, ok := SigilByKey(key)
		if !ok {
			t.Fatalf("SigilByKey(%q) not found", key)
		}
		if !strings.HasSuffix(entry.File, "-wide.svg") {
			t.Fatalf("SigilByKey(%q) file = %q, want the plain face", key, entry.File)
		}
	}
	entry, ok := SigilByKey("sigil-canine-3b82f6-wink")
	if !ok || entry.File != "sigils/canine-3b82f6-wink.svg" {
		t.Fatalf("SigilByKey with an expression = %q, %v", entry.File, ok)
	}
	if _, ok := SigilByKey("sigil-canine-3b82f6-smirk"); ok {
		t.Fatal("an unknown expression should not resolve")
	}
}

// The whole point of the expression axis is that it distinguishes two guests who
// already share a family and a colour, so it has to actually change the drawing.
func TestSigilExpressionsDrawDifferently(t *testing.T) {
	for _, family := range SigilFamilies {
		seen := map[string]string{}
		for _, expression := range SigilExpressions {
			svg, ok := RenderSigil(family, "2f6f3f", expression)
			if !ok {
				t.Fatalf("RenderSigil(%q, %q) failed", family, expression)
			}
			if other, clash := seen[svg]; clash {
				t.Fatalf("%q draws %q and %q identically", family, other, expression)
			}
			seen[svg] = expression
		}
	}
}

// Every family has a face, including the two with no anatomical business having
// one. That is deliberate: the expression is the axis that separates two guests
// who already share an animal and a colour, and a family without one would waste a
// fifth of its keys — a starfish that can wink is worth more here than a starfish
// that is anatomically right.
func TestEveryFamilyHasAFace(t *testing.T) {
	for _, family := range SigilFamilies {
		if eyes, ok := sigilEyes[family]; !ok || len(eyes) == 0 {
			t.Fatalf("%q has no eyes, so its expression would do nothing", family)
		}
	}
	if len(sigilEyes) != len(SigilFamilies) {
		t.Fatalf("sigilEyes has %d entries for %d families", len(sigilEyes), len(SigilFamilies))
	}
}

// The lean is stable for a given avatar and varied across them, so a row of sigils
// does not sit to attention but a guest's own avatar never moves.
func TestSigilTiltIsStableAndVaried(t *testing.T) {
	if sigilTilt("canine3b82f6wink") != sigilTilt("canine3b82f6wink") {
		t.Fatal("tilt is not stable for the same key")
	}
	seen := map[float64]bool{}
	for _, hex := range []string{"2f6f3f", "b32d1e", "1f8f9a", "9a2f8a", "e8e2b0", "123a5c"} {
		for _, expression := range SigilExpressions {
			seen[sigilTilt("canine"+hex+expression)] = true
		}
	}
	if len(seen) < 25 {
		t.Fatalf("tilt only took %d values across 30 keys", len(seen))
	}
	for angle := range seen {
		if angle < -6 || angle > 6 {
			t.Fatalf("tilt of %.2f degrees is outside the intended lean", angle)
		}
	}
}

func TestSigilHandlerRejectsBadPaths(t *testing.T) {
	for _, path := range []string{
		"/avatars/sigils/canine-3b82f6",           // no extension
		"/avatars/sigils/canine-3b82f6-smirk.svg", // unknown expression
		"/avatars/sigils/canine-3b82f6-wide-x.svg",
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
