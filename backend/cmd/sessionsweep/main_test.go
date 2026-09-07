package main

import (
	"testing"
	"time"

	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestResolveThreshold(t *testing.T) {
	cases := []struct {
		name  string
		flag  time.Duration
		env   string
		want  time.Duration
		isErr bool
	}{
		{name: "flag wins over env", flag: 2 * time.Hour, env: "30m", want: 2 * time.Hour},
		{name: "env used when flag unset", env: "90m", want: 90 * time.Minute},
		{name: "default when both unset", want: store.DefaultStaleSessionAge},
		{name: "zero env falls back to default", env: "0s", want: store.DefaultStaleSessionAge},
		{name: "unparseable env is an error", env: "soon", isErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveThreshold(tc.flag, tc.env)
			if tc.isErr {
				if err == nil {
					t.Fatalf("resolveThreshold(%v, %q) = %v, want an error", tc.flag, tc.env, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveThreshold(%v, %q): %v", tc.flag, tc.env, err)
			}
			if got != tc.want {
				t.Fatalf("resolveThreshold(%v, %q) = %v, want %v", tc.flag, tc.env, got, tc.want)
			}
		})
	}
}

// A negative threshold would make every active session stale, including live ones.
func TestResolveThresholdRejectsNegativeFlag(t *testing.T) {
	if _, err := resolveThreshold(-time.Hour, ""); err == nil {
		t.Fatal(`resolveThreshold(-1h, "") = nil error, want a rejection`)
	}
}
