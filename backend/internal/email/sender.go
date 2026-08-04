package email

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// LogSender writes magic links to the application log (default for local dev).
type LogSender struct{}

func (LogSender) SendMagicLink(_ context.Context, msg MagicLinkEmail) error {
	if msg.Code != "" {
		log.Printf("email: sign-in for %s code=%s link=%s", msg.To, msg.Code, msg.Link)
		return nil
	}
	if msg.Link == "" {
		log.Printf("email: sign-in for %s (URL not configured)", msg.To)
		return nil
	}
	log.Printf("email: sign-in for %s -> %s", msg.To, msg.Link)
	return nil
}

// CaptureSender records the last sign-in email for tests.
type CaptureSender struct {
	Last MagicLinkEmail
}

func (c *CaptureSender) SendMagicLink(_ context.Context, msg MagicLinkEmail) error {
	c.Last = msg
	return nil
}

// Config holds SMTP connection settings.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
}

// ConfigFromEnv loads SMTP settings. Returns ok=false when SMTP is not configured.
func ConfigFromEnv() (Config, bool, error) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return Config{}, false, nil
	}

	port := 587
	if rawPort := strings.TrimSpace(os.Getenv("SMTP_PORT")); rawPort != "" {
		parsed, err := strconv.Atoi(rawPort)
		if err != nil {
			return Config{}, false, fmt.Errorf("email: invalid SMTP_PORT %q: %w", rawPort, err)
		}
		port = parsed
	}

	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if from == "" {
		return Config{}, false, fmt.Errorf("email: SMTP_FROM is required when SMTP_HOST is set")
	}

	return Config{
		Host:     host,
		Port:     port,
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     from,
		FromName: strings.TrimSpace(os.Getenv("SMTP_FROM_NAME")),
	}, true, nil
}

// SenderFromEnv returns Resend HTTP API when configured (default), SMTP if RESEND_USE_SMTP=true, otherwise LogSender.
func SenderFromEnv() (Sender, error) {
	cfg, ok, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}

	var sender Sender
	if !ok {
		sender = LogSender{}
	} else if strings.EqualFold(strings.TrimSpace(os.Getenv("RESEND_USE_SMTP")), "true") {
		sender, err = NewSMTPSender(cfg)
		if err != nil {
			return nil, err
		}
	} else {
		apiKey := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
		if apiKey == "" {
			apiKey = strings.TrimSpace(cfg.Password)
		}
		if apiKey != "" {
			sender, err = NewResendSender(ResendConfig{
				APIKey:   apiKey,
				From:     cfg.From,
				FromName: cfg.FromName,
			})
			if err != nil {
				return nil, err
			}
		} else {
			sender, err = NewSMTPSender(cfg)
			if err != nil {
				return nil, err
			}
		}
	}

	if path := strings.TrimSpace(os.Getenv("E2E_SIGNIN_LOG_PATH")); path != "" {
		return E2ESignInLog{Path: path, Inner: sender}, nil
	}

	return sender, nil
}
