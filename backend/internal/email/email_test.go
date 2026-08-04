package email

import (
	"strings"
	"testing"
	"time"
)

func TestSenderFromEnvDefaultsToLogSender(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("E2E_SIGNIN_LOG_PATH", "")

	sender, err := SenderFromEnv()
	if err != nil {
		t.Fatalf("SenderFromEnv() error: %v", err)
	}
	if _, ok := sender.(LogSender); !ok {
		t.Fatalf("expected LogSender, got %T", sender)
	}
}

func TestSenderFromEnvWrapsE2ESignInLog(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("E2E_SIGNIN_LOG_PATH", t.TempDir()+"/sign-in.log")

	sender, err := SenderFromEnv()
	if err != nil {
		t.Fatalf("SenderFromEnv() error: %v", err)
	}
	if _, ok := sender.(E2ESignInLog); !ok {
		t.Fatalf("expected E2ESignInLog, got %T", sender)
	}
}

func TestConfigFromEnvRequiresFromAddress(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "")

	_, ok, err := ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error when SMTP_FROM is missing")
	}
	if ok {
		t.Fatal("expected ok=false")
	}
}

func TestFormatTTL(t *testing.T) {
	tests := []struct {
		ttl  time.Duration
		want string
	}{
		{59 * time.Second, "less than a minute"},
		{30 * time.Second, "less than a minute"},
		{time.Minute, "1 minute"},
		{15 * time.Minute, "15 minutes"},
	}

	for _, tc := range tests {
		if got := formatTTL(tc.ttl); got != tc.want {
			t.Fatalf("formatTTL(%v) = %q, want %q", tc.ttl, got, tc.want)
		}
	}
}

func TestMagicLinkBodyIncludesLinkAndTTL(t *testing.T) {
	body := magicLinkBody(MagicLinkEmail{
		To:   "player@example.com",
		Link: "http://localhost:5173/auth/complete?token=abc",
		TTL:  15 * time.Minute,
	})

	if !strings.Contains(body, "http://localhost:5173/auth/complete?token=abc") {
		t.Fatalf("expected link in body: %q", body)
	}
	if !strings.Contains(body, "15 minutes") {
		t.Fatalf("expected ttl in body: %q", body)
	}
	if !strings.Contains(body, "JoinQuest") {
		t.Fatalf("expected branding in body: %q", body)
	}
}

func TestMagicLinkBodyIncludesLoginCode(t *testing.T) {
	body := magicLinkBody(MagicLinkEmail{
		To:   "player@example.com",
		Link: "https://joinquest.cc/auth/complete?token=abc",
		Code: "123456",
		TTL:  15 * time.Minute,
	})

	if !strings.Contains(body, "123456") {
		t.Fatalf("expected code in body: %q", body)
	}
	if !strings.Contains(body, "Your sign-in code:") {
		t.Fatalf("expected code label in body: %q", body)
	}
}
