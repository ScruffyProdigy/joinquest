package avatars

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// SourceSigil marks the guest-tier avatars picked from the first-entry overlay.
// They stay deliberately plainer than SPIRIT_ANIMAL avatars, which are earned by
// signing in and finishing the spirit animal journey.
const SourceSigil = "sigil"

// A sigil is drawn in one of two marks: near-white on a dark disc, near-black on a
// light one. Carrying both is what lets the disc use the whole lightness range
// instead of only the dark end a pale mark can sit on.
const (
	sigilPale = "#f8fafc"
	sigilInk  = "#10141a"
)

// sigilMarkCrossover is the disc luminance above which the dark mark contrasts
// better than the pale one. Picking the better of the two puts every sigil at
// 4.2:1 or above — comfortably past the 3:1 WCAG floor for non-text contrast, and
// better than the pale mark alone managed.
const sigilMarkCrossover = 0.189

// hexLuminance is the WCAG relative luminance of a bare 6-digit hex.
func hexLuminance(hex string) float64 {
	channel := func(i int) float64 {
		v, err := strconv.ParseUint(hex[i:i+2], 16, 16)
		if err != nil {
			return 0
		}
		f := float64(v) / 255
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(0) + 0.7152*channel(2) + 0.0722*channel(4)
}

// sigilMark picks the colour for both the silhouette and the rim: whichever of the
// two reads more strongly on this disc. They share a colour on purpose. The rim is
// what keeps the disc's edge visible when a game client draws it on a background we
// do not control, and the edge needs help in opposite directions at each end — a
// dark disc disappears on a black page, a pale one on a white page — which is
// exactly how the mark already differs.
func sigilMark(hex string) string {
	if hexLuminance(hex) > sigilMarkCrossover {
		return sigilInk
	}
	return sigilPale
}

// SigilFamilies are the guest-tier silhouettes. A generated guest name picks the
// noun that matches its family, so a player called FrostFox gets the canine one.
// The list is kept in step with SIGIL_FAMILIES in frontend/src/lib/guestIdentity.js.
var SigilFamilies = []string{
	"canine", "feline", "horned", "raptor", "corvid", "ursine",
	"rodent", "lagomorph", "serpent", "cephalopod", "cetacean", "chelonian",
	"equine", "proboscid", "suid", "primate", "amphibian", "crustacean",
	"arachnid", "waterfowl", "lepidopteran", "echinoderm", "gastropod", "spheniscid",
}

// legacySigilTints are the twelve named colours the picker offered before it
// started drawing its own colour per guest. Rows saved back then still hold keys
// like "sigil-canine-frost", so the names stay resolvable and render the same.
var legacySigilTints = map[string]string{
	"frost": "0284c7",
	"ember": "ea580c",
	"blaze": "dc2626",
	"dawn":  "e11d48",
	"dusk":  "9333ea",
	"storm": "4f46e5",
	"moss":  "16a34a",
	"tide":  "0d9488",
	"solar": "ca8a04",
	"nova":  "0891b2",
	"rust":  "d97706",
	"bloom": "db2777",
}

var sigilHexPattern = regexp.MustCompile(`^[0-9a-f]{6}$`)

// sigilTint resolves the colour half of a sigil key to a bare 6-digit hex.
// It accepts either a colour the picker drew ("3b82f6") or one of the legacy
// tint names ("frost").
func sigilTint(tint string) (string, bool) {
	if sigilHexPattern.MatchString(tint) {
		return tint, true
	}
	hex, ok := legacySigilTints[tint]
	return hex, ok
}

// SigilByKey resolves a "sigil-<family>-<colour>" key, e.g. "sigil-canine-3b82f6"
// or the older "sigil-canine-frost". Keys are composite rather than a flat catalog
// because the colour is drawn per guest; SigilHandler renders the matching file.
func SigilByKey(key string) (StarterEntry, bool) {
	normalized := strings.ToLower(strings.TrimSpace(key))
	rest, ok := strings.CutPrefix(normalized, "sigil-")
	if !ok {
		return StarterEntry{}, false
	}
	family, tint, ok := strings.Cut(rest, "-")
	if !ok || !slices.Contains(SigilFamilies, family) {
		return StarterEntry{}, false
	}
	hex, ok := sigilTint(tint)
	if !ok {
		return StarterEntry{}, false
	}
	return StarterEntry{
		Key:  normalized,
		Name: family,
		Slot: family,
		File: fmt.Sprintf("sigils/%s-%s.svg", family, hex),
	}, true
}

// RenderSigil draws one sigil: the animal silhouette for `family` on a disc of
// `tint`, with the cut-out details punched back through in the disc colour.
func RenderSigil(family, tint string) (string, bool) {
	shape, ok := sigilShapes[family]
	if !ok {
		return "", false
	}
	hex, ok := sigilTint(tint)
	if !ok {
		return "", false
	}
	colour := "#" + hex
	mark := sigilMark(hex)
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64" role="img" aria-label="%s sigil">
  <circle cx="32" cy="32" r="32" fill="%s"/>
  <circle cx="32" cy="32" r="30.8" fill="none" stroke="%s" stroke-width="2.4" opacity="0.5"/>%s
</svg>
`, family, colour, mark, shape(mark, colour)), true
}

// SigilHandler serves /avatars/sigils/<family>-<colour>.svg. Sigils are rendered
// rather than stored because the colour is drawn fresh for each guest, so there is
// no fixed set of files to ship. The colour is baked into the SVG rather than
// applied with CSS because game clients render `avatarUrl` on their own
// backgrounds, where a transparent silhouette could land invisible.
func SigilHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/avatars/sigils/"))
		name, ok := strings.CutSuffix(name, ".svg")
		if !ok {
			http.NotFound(w, r)
			return
		}
		family, tint, ok := strings.Cut(name, "-")
		if !ok {
			http.NotFound(w, r)
			return
		}
		svg, ok := RenderSigil(family, tint)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		// The URL fully determines the image, so it never needs revalidating.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fmt.Fprint(w, svg)
	})
}

// sigilShapes draws each family's head in `body`, with details cut back out in
// `tint` so they read as holes in the silhouette rather than added ink.
var sigilShapes = map[string]func(body, tint string) string{
	"canine": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M19 12 L27 22 L21 28 Z" fill="%[1]s"/>
  <path d="M45 12 L37 22 L43 28 Z" fill="%[1]s"/>
  <path d="M32 18 C41 18 48 25 48 33 C48 38 45 42 41 44 L23 44 C19 42 16 38 16 33 C16 25 23 18 32 18 Z" fill="%[1]s"/>
  <path d="M25 41 L39 41 L37 49 C36 52 28 52 27 49 Z" fill="%[1]s"/>
  <circle cx="25" cy="31" r="2.4" fill="%[2]s"/>
  <circle cx="39" cy="31" r="2.4" fill="%[2]s"/>
  <ellipse cx="32" cy="46" rx="2.8" ry="2.1" fill="%[2]s"/>`, body, tint)
	},
	"feline": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M17 18 L24 27 L40 27 L47 18 L48 31 C48 42 41 49 32 49 C23 49 16 42 16 31 Z" fill="%[1]s"/>
  <circle cx="25" cy="33" r="2.4" fill="%[2]s"/>
  <circle cx="39" cy="33" r="2.4" fill="%[2]s"/>
  <path d="M32 39 L29 42 H35 Z" fill="%[2]s"/>`, body, tint)
	},
	"horned": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M26 24 C18 23 12 19 9 11 C13 21 18 28 26 30 Z" fill="%[1]s"/>
  <path d="M38 24 C46 23 52 19 55 11 C51 21 46 28 38 30 Z" fill="%[1]s"/>
  <path d="M24 21 H40 L42 33 C42 42 38 49 32 49 C26 49 22 42 22 33 Z" fill="%[1]s"/>
  <circle cx="27" cy="31" r="2.4" fill="%[2]s"/>
  <circle cx="37" cy="31" r="2.4" fill="%[2]s"/>
  <ellipse cx="32" cy="43" rx="4.5" ry="3.2" fill="%[2]s"/>`, body, tint)
	},
	"raptor": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M30 23 C22 20 12 20 5 24 C11 26 20 29 26 35 L31 31 Z" fill="%[1]s"/>
  <path d="M34 23 C42 20 52 20 59 24 C53 26 44 29 38 35 L33 31 Z" fill="%[1]s"/>
  <ellipse cx="32" cy="28" rx="4" ry="11" fill="%[1]s"/>
  <circle cx="32" cy="18" r="3.6" fill="%[1]s"/>
  <path d="M28 36 L36 36 L38 49 L32 45 L26 49 Z" fill="%[1]s"/>
  <path d="M12 24 L20 27 M52 24 L44 27" fill="none" stroke="%[2]s" stroke-width="1.8" stroke-linecap="round"/>
  <path d="M28 42 L36 42" fill="none" stroke="%[2]s" stroke-width="2" stroke-linecap="round"/>`, body, tint)
	},
	"corvid": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M18 28 C18 19 25 14 32 16 L57 27 L34 32 C35 40 30 47 24 46 C18 45 16 37 18 28 Z" fill="%[1]s"/>
  <circle cx="27" cy="26" r="2.8" fill="%[2]s"/>`, body, tint)
	},
	"ursine": func(body, tint string) string {
		return fmt.Sprintf(`
  <circle cx="19" cy="21" r="7.5" fill="%[1]s"/>
  <circle cx="45" cy="21" r="7.5" fill="%[1]s"/>
  <path d="M14 34 C14 24 22 18 32 18 C42 18 50 24 50 34 C50 44 42 50 32 50 C22 50 14 44 14 34 Z" fill="%[1]s"/>
  <circle cx="25" cy="31" r="2.4" fill="%[2]s"/>
  <circle cx="39" cy="31" r="2.4" fill="%[2]s"/>
  <ellipse cx="32" cy="41" rx="6" ry="4.5" fill="%[2]s"/>`, body, tint)
	},
	"rodent": func(body, tint string) string {
		return fmt.Sprintf(`
  <circle cx="17" cy="21" r="8.5" fill="%[1]s"/>
  <circle cx="47" cy="21" r="8.5" fill="%[1]s"/>
  <circle cx="17" cy="21" r="4" fill="%[2]s"/>
  <circle cx="47" cy="21" r="4" fill="%[2]s"/>
  <path d="M32 17 C41 17 48 24 48 32 C48 40 40 45 34 52 L32 54 L30 52 C24 45 16 40 16 32 C16 24 23 17 32 17 Z" fill="%[1]s"/>
  <circle cx="25" cy="32" r="2.4" fill="%[2]s"/>
  <circle cx="39" cy="32" r="2.4" fill="%[2]s"/>
  <circle cx="32" cy="45" r="2.6" fill="%[2]s"/>`, body, tint)
	},
	"lagomorph": func(body, tint string) string {
		return fmt.Sprintf(`
  <ellipse cx="24" cy="18" rx="5.5" ry="13" fill="%[1]s"/>
  <ellipse cx="40" cy="18" rx="5.5" ry="13" fill="%[1]s"/>
  <ellipse cx="24" cy="19" rx="2.2" ry="8.5" fill="%[2]s"/>
  <ellipse cx="40" cy="19" rx="2.2" ry="8.5" fill="%[2]s"/>
  <path d="M32 26 C41 26 48 33 48 41 C48 48 41 53 32 53 C23 53 16 48 16 41 C16 33 23 26 32 26 Z" fill="%[1]s"/>
  <circle cx="25" cy="39" r="2.4" fill="%[2]s"/>
  <circle cx="39" cy="39" r="2.4" fill="%[2]s"/>
  <path d="M32 48 L28.5 44 H35.5 Z" fill="%[2]s"/>`, body, tint)
	},
	"serpent": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M15 46 C15 38 25 35 32 39 C38 42 44 40 44 34 C44 29 40 26 35 28" fill="none" stroke="%[1]s" stroke-width="8" stroke-linecap="round"/>
  <path d="M32 32 C27 26 30 18 37 16 L53 20 L42 27 C38 30 34 34 32 32 Z" fill="%[1]s"/>
  <circle cx="41" cy="22" r="2.2" fill="%[2]s"/>
  <path d="M52 19 L59 15 M52 22 L59 25" fill="none" stroke="%[2]s" stroke-width="2.4" stroke-linecap="round"/>`, body, tint)
	},
	"cephalopod": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M32 13 C43 13 51 22 51 33 C51 38 49 43 47 46 L45 39 L42 47 L39 40 L36 48 L32 41 L28 48 L25 40 L22 47 L19 39 L17 46 C15 43 13 38 13 33 C13 22 21 13 32 13 Z" fill="%[1]s"/>
  <circle cx="24" cy="31" r="3.2" fill="%[2]s"/>
  <circle cx="40" cy="31" r="3.2" fill="%[2]s"/>`, body, tint)
	},
	"cetacean": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M13 24 L6 17 L8 32 L6 47 L13 40 Z" fill="%[1]s"/>
  <path d="M25 7 C31 9 35 12 38 17 L26 19 Z" fill="%[1]s"/>
  <path d="M53 32 C53 40 44 47 32 47 C24 47 17 44 13 40 L13 24 C17 20 24 17 32 17 C44 17 53 24 53 32 Z" fill="%[1]s"/>
  <circle cx="44" cy="28" r="2.4" fill="%[2]s"/>
  <path d="M34 45 C39 43 44 40 48 35" fill="none" stroke="%[2]s" stroke-width="2" stroke-linecap="round"/>`, body, tint)
	},
	"chelonian": func(body, tint string) string {
		return fmt.Sprintf(`
  <circle cx="32" cy="13" r="7.5" fill="%[1]s"/>
  <ellipse cx="15" cy="24" rx="6.5" ry="4.5" transform="rotate(-40 15 24)" fill="%[1]s"/>
  <ellipse cx="49" cy="24" rx="6.5" ry="4.5" transform="rotate(40 49 24)" fill="%[1]s"/>
  <ellipse cx="16" cy="46" rx="6" ry="4.2" transform="rotate(40 16 46)" fill="%[1]s"/>
  <ellipse cx="48" cy="46" rx="6" ry="4.2" transform="rotate(-40 48 46)" fill="%[1]s"/>
  <ellipse cx="32" cy="34" rx="17" ry="15" fill="%[1]s"/>
  <ellipse cx="32" cy="34" rx="12" ry="10" fill="none" stroke="%[2]s" stroke-width="2.2"/>
  <path d="M32 28 L37 31 L37 37 L32 40 L27 37 L27 31 Z" fill="%[2]s"/>`, body, tint)
	},
	"equine": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M24 8 C20 13 21 21 26 25 C30 21 28 11 24 8 Z" fill="%[1]s"/>
  <path d="M40 8 C44 13 43 21 38 25 C34 21 36 11 40 8 Z" fill="%[1]s"/>
  <path d="M24 22 C24 18 40 18 40 22 L41 31 C41 37 39 43 36 47 C34 50 30 50 28 47 C25 43 23 37 23 31 Z" fill="%[1]s"/>
  <circle cx="27.5" cy="27" r="2.2" fill="%[2]s"/>
  <circle cx="36.5" cy="27" r="2.2" fill="%[2]s"/>
  <ellipse cx="29.5" cy="43" rx="1.8" ry="2.5" fill="%[2]s"/>
  <ellipse cx="34.5" cy="43" rx="1.8" ry="2.5" fill="%[2]s"/>`, body, tint)
	},
	"proboscid": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M22 16 C9 11 2 19 3 30 C4 41 14 47 23 44 Z" fill="%[1]s"/>
  <path d="M42 16 C55 11 62 19 61 30 C60 41 50 47 41 44 Z" fill="%[1]s"/>
  <path d="M32 12 C40 12 45 18 45 26 L45 32 C45 37 41 40 36 40 L28 40 C23 40 19 37 19 32 L19 26 C19 18 24 12 32 12 Z" fill="%[1]s"/>
  <path d="M29 37 L35 37 L34 52 C34 56 30 56 30 52 Z" fill="%[1]s"/>
  <path d="M25 39 C23 44 22 48 21 52 C24 49 26 45 27 41 Z" fill="%[1]s"/>
  <path d="M39 39 C41 44 42 48 43 52 C40 49 38 45 37 41 Z" fill="%[1]s"/>
  <circle cx="25" cy="25" r="2.4" fill="%[2]s"/>
  <circle cx="39" cy="25" r="2.4" fill="%[2]s"/>`, body, tint)
	},
	"suid": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M17 16 L25 22 L18 27 Z" fill="%[1]s"/>
  <path d="M47 16 L39 22 L46 27 Z" fill="%[1]s"/>
  <path d="M32 17 C42 17 50 24 50 33 C50 42 42 49 32 49 C22 49 14 42 14 33 C14 24 22 17 32 17 Z" fill="%[1]s"/>
  <path d="M22 43 C16 40 12 34 11 28 C13 35 17 41 21 45 Z" fill="%[1]s"/>
  <path d="M42 43 C48 40 52 34 53 28 C51 35 47 41 43 45 Z" fill="%[1]s"/>
  <circle cx="24" cy="30" r="2.4" fill="%[2]s"/>
  <circle cx="40" cy="30" r="2.4" fill="%[2]s"/>
  <ellipse cx="32" cy="40" rx="7.5" ry="5.5" fill="%[2]s"/>
  <ellipse cx="29" cy="40" rx="1.6" ry="2.3" fill="%[1]s"/>
  <ellipse cx="35" cy="40" rx="1.6" ry="2.3" fill="%[1]s"/>`, body, tint)
	},
	"primate": func(body, tint string) string {
		return fmt.Sprintf(`
  <circle cx="14" cy="30" r="6.5" fill="%[1]s"/>
  <circle cx="50" cy="30" r="6.5" fill="%[1]s"/>
  <path d="M32 13 C42 13 49 21 49 31 C49 42 42 50 32 50 C22 50 15 42 15 31 C15 21 22 13 32 13 Z" fill="%[1]s"/>
  <path d="M22 24 C26 20 30 19 32 19 C34 19 38 20 42 24 L42 27 C37 24 34 23 32 23 C30 23 27 24 22 27 Z" fill="%[2]s"/>
  <circle cx="26" cy="31" r="2.4" fill="%[2]s"/>
  <circle cx="38" cy="31" r="2.4" fill="%[2]s"/>
  <ellipse cx="32" cy="41" rx="8" ry="6" fill="%[2]s"/>
  <path d="M28 41 L36 41" fill="none" stroke="%[1]s" stroke-width="2" stroke-linecap="round"/>`, body, tint)
	},
	"amphibian": func(body, tint string) string {
		return fmt.Sprintf(`
  <circle cx="20" cy="21" r="8" fill="%[1]s"/>
  <circle cx="44" cy="21" r="8" fill="%[1]s"/>
  <path d="M10 34 C10 26 20 21 32 21 C44 21 54 26 54 34 C54 42 44 48 32 48 C20 48 10 42 10 34 Z" fill="%[1]s"/>
  <circle cx="20" cy="20" r="3.6" fill="%[2]s"/>
  <circle cx="44" cy="20" r="3.6" fill="%[2]s"/>
  <path d="M17 37 C23 43 41 43 47 37" fill="none" stroke="%[2]s" stroke-width="2.6" stroke-linecap="round"/>`, body, tint)
	},
	"crustacean": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M20 42 L12 49 M26 45 L22 53 M44 42 L52 49 M38 45 L42 53" fill="none" stroke="%[1]s" stroke-width="3.4" stroke-linecap="round"/>
  <path d="M22 28 C16 22 9 19 5 22 C1 25 3 33 9 35 C14 37 19 33 22 31 Z" fill="%[1]s"/>
  <path d="M42 28 C48 22 55 19 59 22 C63 25 61 33 55 35 C50 37 45 33 42 31 Z" fill="%[1]s"/>
  <path d="M4 24 L14 29 L4 33 Z" fill="%[2]s"/>
  <path d="M60 24 L50 29 L60 33 Z" fill="%[2]s"/>
  <ellipse cx="32" cy="37" rx="14" ry="10" fill="%[1]s"/>
  <circle cx="26" cy="34" r="2.6" fill="%[2]s"/>
  <circle cx="38" cy="34" r="2.6" fill="%[2]s"/>
  <path d="M28 43 L36 43" fill="none" stroke="%[2]s" stroke-width="2.2" stroke-linecap="round"/>`, body, tint)
	},
	"arachnid": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M26 25 C19 21 15 18 12 15 M25 28 C17 27 12 26 8 24 M25 32 C17 33 12 35 9 38 M27 35 C22 39 18 43 15 47" fill="none" stroke="%[1]s" stroke-width="2.6" stroke-linecap="round"/>
  <path d="M38 25 C45 21 49 18 52 15 M39 28 C47 27 52 26 56 24 M39 32 C47 33 52 35 55 38 M37 35 C42 39 46 43 49 47" fill="none" stroke="%[1]s" stroke-width="2.6" stroke-linecap="round"/>
  <ellipse cx="32" cy="39" rx="11" ry="10" fill="%[1]s"/>
  <ellipse cx="32" cy="26" rx="8" ry="7" fill="%[1]s"/>
  <circle cx="29" cy="24" r="2" fill="%[2]s"/>
  <circle cx="35" cy="24" r="2" fill="%[2]s"/>`, body, tint)
	},
	"waterfowl": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M27 40 L25 53 M34 40 L37 53" fill="none" stroke="%[1]s" stroke-width="2.6" stroke-linecap="round"/>
  <path d="M21 54 L30 54 M33 54 L42 54" fill="none" stroke="%[1]s" stroke-width="2.4" stroke-linecap="round"/>
  <path d="M18 31 L7 26 L16 38 Z" fill="%[1]s"/>
  <ellipse cx="30" cy="33" rx="13" ry="8.5" fill="%[1]s"/>
  <path d="M36 29 C33 23 34 17 39 14" fill="none" stroke="%[1]s" stroke-width="5" stroke-linecap="round"/>
  <circle cx="41" cy="13" r="4.4" fill="%[1]s"/>
  <path d="M45 11 L57 15 L45 16 Z" fill="%[1]s"/>
  <circle cx="42" cy="12" r="1.8" fill="%[2]s"/>
  <path d="M23 32 C27 27 35 27 39 32 C35 37 27 37 23 32 Z" fill="%[2]s"/>`, body, tint)
	},
	"lepidopteran": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M31 22 C28 14 20 8 13 11 C6 14 5 24 14 30 C6 33 4 44 11 49 C18 53 28 45 31 36 Z" fill="%[1]s"/>
  <path d="M33 22 C36 14 44 8 51 11 C58 14 59 24 50 30 C58 33 60 44 53 49 C46 53 36 45 33 36 Z" fill="%[1]s"/>
  <path d="M31 20 C29 15 26 12 23 10 M33 20 C35 15 38 12 41 10" fill="none" stroke="%[1]s" stroke-width="2" stroke-linecap="round"/>
  <ellipse cx="32" cy="32" rx="2.8" ry="13" fill="%[1]s"/>
  <circle cx="17" cy="22" r="3.6" fill="%[2]s"/>
  <circle cx="47" cy="22" r="3.6" fill="%[2]s"/>`, body, tint)
	},
	"echinoderm": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M32 9 L37.6 24.3 L53.9 24.9 L41 34.9 L45.5 50.6 L32 41.5 L18.5 50.6 L23 34.9 L10.1 24.9 L26.4 24.3 Z" fill="%[1]s"/>
  <circle cx="32" cy="31" r="4.2" fill="%[2]s"/>
  <circle cx="32" cy="18" r="1.7" fill="%[2]s"/>
  <circle cx="43" cy="27" r="1.7" fill="%[2]s"/>
  <circle cx="39" cy="41" r="1.7" fill="%[2]s"/>
  <circle cx="25" cy="41" r="1.7" fill="%[2]s"/>
  <circle cx="21" cy="27" r="1.7" fill="%[2]s"/>`, body, tint)
	},
	"gastropod": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M9 45 C9 41 13 38 19 38 L45 38 C51 38 55 41 55 45 C55 47 53 49 51 49 L13 49 C11 49 9 47 9 45 Z" fill="%[1]s"/>
  <path d="M46 39 C50 34 52 30 52 26 M40 38 C43 34 45 31 45 28" fill="none" stroke="%[1]s" stroke-width="2.8" stroke-linecap="round"/>
  <circle cx="52" cy="25" r="2.6" fill="%[1]s"/>
  <circle cx="45" cy="27" r="2.4" fill="%[1]s"/>
  <circle cx="27" cy="27" r="15" fill="%[1]s"/>
  <circle cx="27" cy="27" r="10.5" fill="%[2]s"/>
  <circle cx="27" cy="27" r="6.5" fill="%[1]s"/>
  <circle cx="27" cy="27" r="2.8" fill="%[2]s"/>`, body, tint)
	},
	"spheniscid": func(body, tint string) string {
		return fmt.Sprintf(`
  <path d="M19 30 C13 33 11 40 13 46 C15 43 17 38 20 35 Z" fill="%[1]s"/>
  <path d="M45 30 C51 33 53 40 51 46 C49 43 47 38 44 35 Z" fill="%[1]s"/>
  <path d="M24 53 L20 57 H28 Z" fill="%[1]s"/>
  <path d="M40 53 L44 57 H36 Z" fill="%[1]s"/>
  <path d="M32 11 C40 11 45 18 45 26 C45 29 44 32 43 34 L45 46 C46 51 40 55 32 55 C24 55 18 51 19 46 L21 34 C20 32 19 29 19 26 C19 18 24 11 32 11 Z" fill="%[1]s"/>
  <circle cx="27" cy="22" r="2.1" fill="%[2]s"/>
  <circle cx="37" cy="22" r="2.1" fill="%[2]s"/>
  <path d="M32 25 L28 28.5 L32 32 L36 28.5 Z" fill="%[2]s"/>`, body, tint)
	},
}
