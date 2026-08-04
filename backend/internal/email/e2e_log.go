package email

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// E2ESignInLog mirrors sign-in emails to a file so Playwright tests can read codes immediately.
type E2ESignInLog struct {
	Path  string
	Inner Sender
}

func (s E2ESignInLog) SendMagicLink(ctx context.Context, msg MagicLinkEmail) error {
	if err := s.Inner.SendMagicLink(ctx, msg); err != nil {
		return err
	}

	path := strings.TrimSpace(s.Path)
	if path == "" {
		return nil
	}

	line := fmt.Sprintf("email: sign-in for %s code=%s link=%s\n", msg.To, msg.Code, msg.Link)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil
	}
	defer file.Close()

	_, _ = file.WriteString(line)
	_ = file.Sync()
	return nil
}
