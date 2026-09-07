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
// row of avatars reads as lit from one place, and by an amount that tapers to
// nothing in the middle of the lightness range — which is what keeps the lift from
// stranding the mark on discs that have no room for it.
func TestSigilGradientLiftTapersTowardTheMiddle(t *testing.T) {
	spread := func(hex string) float64 {
		from, to := sigilGradient(hex)
		a := lightnessStar(luminance(parseHex(strings.TrimPrefix(from, "#"))))
		b := lightnessStar(luminance(parseHex(strings.TrimPrefix(to, "#"))))
		return a - b
	}
	for _, hex := range []string{"0a2312", "c04a2a", "9ad4a8", "e8e2b0", "f2f6f3"} {
		baseL := lightnessStar(luminance(parseHex(hex)))
		got := spread(hex)
		if got <= 0 {
			t.Fatalf("gradient on %q does not lift its first stop: %.1f L*", hex, got)
		}
		// A disc close to black or white has one stop clamped at the end of the
		// scale, so the expected spread is what survives the clamp.
		half := gradientHalfLift(baseL)
		want := math.Min(100, baseL+half) - math.Max(0, baseL-half)
		if math.Abs(got-want) > 1.5 {
			t.Fatalf("gradient on %q lifts by %.1f L*, want %.1f", hex, got, want)
		}
	}

	// A disc sitting at the middle gets essentially no lift, and one at the edge
	// gets the full amount.
	middle := formatHex(hslToRGB(150, 70, lightnessForLuminance(150, 70, luminanceForLightnessStar(discLightnessMid))))
	if got := spread(strings.TrimPrefix(middle, "#")); got > 1.0 {
		t.Fatalf("a mid-lightness disc should barely lift, got %.1f L*", got)
	}
	edge := formatHex(hslToRGB(150, 70, lightnessForLuminance(150, 70, luminanceForLightnessStar(88))))
	if got := spread(strings.TrimPrefix(edge, "#")); got < gradientLift*0.8 {
		t.Fatalf("a disc at the edge should lift fully, got %.1f L*", got)
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
	for family := range sigilEyes {
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

// Five families have no eyes to put an expression on — a raptor seen from below,
// a starfish. Their keys still carry one, and it still separates them in the key,
// but the drawing is the same. This is a known gap, pinned so it stays deliberate.
func TestEyelessFamiliesIgnoreTheExpression(t *testing.T) {
	eyeless := []string{"raptor", "chelonian", "lepidopteran", "echinoderm", "gastropod"}
	for _, family := range eyeless {
		if _, ok := sigilEyes[family]; ok {
			t.Fatalf("%q has eyes now — give it expressions and drop it from this list", family)
		}
		first, _ := RenderSigil(family, "2f6f3f", SigilExpressions[0])
		for _, expression := range SigilExpressions[1:] {
			svg, _ := RenderSigil(family, "2f6f3f", expression)
			// The gradient id carries the expression, so compare the drawing itself.
			if strings.Count(svg, "<circle") != strings.Count(first, "<circle") ||
				strings.Count(svg, "<path") != strings.Count(first, "<path") {
				t.Fatalf("%q unexpectedly changed shape for %q", family, expression)
			}
		}
	}
	if len(sigilEyes)+len(eyeless) != len(SigilFamilies) {
		t.Fatalf("%d families have eyes and %d are listed eyeless, but there are %d",
			len(sigilEyes), len(eyeless), len(SigilFamilies))
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
