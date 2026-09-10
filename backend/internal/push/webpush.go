package push

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// DefaultTTL is a backstop for callers that set no TTL, so a forgotten one is
// bounded rather than left to the push service's own retention (days).
// Seat-bound notifications must pass an explicit TTL instead.
const DefaultTTL = 5 * time.Minute

// WebPushSender is the real sender, signing payloads with VAPID.
type WebPushSender struct {
	publicKey  string
	privateKey string
	subject    string
	client     *http.Client
}

// Config holds VAPID application server credentials.
type Config struct {
	PublicKey  string
	PrivateKey string
	// Subject is the mailto: or https: contact URL the VAPID spec requires.
	Subject string
}

// ConfigFromEnv loads VAPID settings. ok=false when push is unconfigured, the
// normal state in local development.
func ConfigFromEnv() (Config, bool) {
	cfg := Config{
		PublicKey:  strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY")),
		PrivateKey: strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY")),
		Subject:    strings.TrimSpace(os.Getenv("VAPID_SUBJECT")),
	}
	if cfg.PublicKey == "" || cfg.PrivateKey == "" {
		return Config{}, false
	}
	if cfg.Subject == "" {
		cfg.Subject = "mailto:support@joinquest.cc"
	}
	return cfg, true
}

// NewWebPushSender builds a sender from VAPID credentials.
func NewWebPushSender(cfg Config) *WebPushSender {
	return &WebPushSender{
		publicKey:  cfg.PublicKey,
		privateKey: cfg.PrivateKey,
		subject:    cfg.Subject,
		// Bounded: this runs inside match formation, so a hanging push service
		// must not stall the match for the players who are present.
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// PublicKey returns the VAPID application server key.
func (s *WebPushSender) PublicKey() string {
	if s == nil {
		return ""
	}
	return s.publicKey
}

// payload is the wire shape frontend/public/sw.js parses.
type payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

// Send delivers one notification. A permanent rejection becomes
// ErrSubscriptionGone.
func (s *WebPushSender) Send(ctx context.Context, sub Subscription, note Notification) error {
	if s == nil || s.publicKey == "" || s.privateKey == "" {
		return fmt.Errorf("push: sender not configured")
	}

	body, err := json.Marshal(payload{
		Title: note.Title,
		Body:  note.Body,
		URL:   note.URL,
		Tag:   note.Tag,
	})
	if err != nil {
		return err
	}

	ttl := note.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}

	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		Subscriber:      s.subject,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		TTL:             int(ttl.Seconds()),
		HTTPClient:      s.client,
		// High urgency: the seat has a deadline, so the push must not be
		// batched by the service.
		Urgency: webpush.UrgencyHigh,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	// 404/410 mean the subscription is permanently gone. Anything else is
	// transient, so the row stays.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return ErrSubscriptionGone
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push: push service returned %d", resp.StatusCode)
	}
	return nil
}

// LogSender is the local-development sender. Its empty public key tells the
// frontend push is unavailable.
type LogSender struct{}

// Send logs the notification.
func (LogSender) Send(_ context.Context, sub Subscription, note Notification) error {
	log.Printf("push: would notify endpoint=%s title=%q url=%s", sub.Endpoint, note.Title, note.URL)
	return nil
}

// PublicKey reports that push is unconfigured.
func (LogSender) PublicKey() string { return "" }

// SenderFromEnv returns the configured sender, falling back to LogSender.
func SenderFromEnv() Sender {
	if cfg, ok := ConfigFromEnv(); ok {
		log.Printf("push: web push enabled (subject=%s)", cfg.Subject)
		return NewWebPushSender(cfg)
	}
	log.Printf("push: VAPID keys not configured; notifications will be logged only")
	return LogSender{}
}
