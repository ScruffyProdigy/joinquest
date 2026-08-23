package developer

import (
	"fmt"
	"regexp"
	"strings"
)

var accentColorPattern = regexp.MustCompile(`^#([0-9a-f]{3}|[0-9a-f]{6})$`)

// NormalizeAccentColor validates a developer-supplied catalog accent color and
// returns it as lowercase #rrggbb, expanding #rgb shorthand. Callers treat an
// empty input as "clear the override" before calling this.
func NormalizeAccentColor(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if !accentColorPattern.MatchString(value) {
		return "", fmt.Errorf("accent color %q must be a hex color like #7c3aed", raw)
	}
	if len(value) == 4 {
		return fmt.Sprintf("#%c%c%c%c%c%c",
			value[1], value[1], value[2], value[2], value[3], value[3]), nil
	}
	return value, nil
}
