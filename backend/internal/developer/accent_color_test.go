package developer

import "testing"

func TestNormalizeAccentColor(t *testing.T) {
	valid := map[string]string{
		"#7c3aed":   "#7c3aed",
		"#7C3AED":   "#7c3aed",
		"  #7c3aed": "#7c3aed",
		"#abc":      "#aabbcc",
		"#ABC":      "#aabbcc",
	}
	for raw, want := range valid {
		got, err := NormalizeAccentColor(raw)
		if err != nil {
			t.Fatalf("NormalizeAccentColor(%q): unexpected error %v", raw, err)
		}
		if got != want {
			t.Fatalf("NormalizeAccentColor(%q) = %q, want %q", raw, got, want)
		}
	}

	invalid := []string{"", "7c3aed", "#12345", "#1234567", "#gggggg", "rebeccapurple", "rgb(1,2,3)"}
	for _, raw := range invalid {
		if _, err := NormalizeAccentColor(raw); err == nil {
			t.Fatalf("NormalizeAccentColor(%q): expected an error", raw)
		}
	}
}
