package push

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// newTestSender builds a sender with throwaway VAPID keys, pointed at a fake
// push service.
func newTestSender(t *testing.T) *WebPushSender {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys: %v", err)
	}
	return NewWebPushSender(Config{PublicKey: pub, PrivateKey: priv, Subject: "mailto:test@example.com"})
}

// testSubscription is a syntactically valid subscription. The keys are the
// browser's ECDH public key and auth secret; the encryption only needs them to
// be well-formed base64url of the right length, not to belong to a real client.
func testSubscription(endpoint string) Subscription {
	return Subscription{
		Endpoint: endpoint,
		P256dh:   "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM",
		Auth:     "tBHItJI5svbpez7KI4CCXg",
	}
}

func TestSendDeliversTheNotificationToThePushService(t *testing.T) {
	var gotTTL, gotUrgency, gotEncoding string
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotTTL = r.Header.Get("TTL")
		gotUrgency = r.Header.Get("Urgency")
		gotEncoding = r.Header.Get("Content-Encoding")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sender := newTestSender(t)
	err := sender.Send(context.Background(), testSubscription(server.URL), Notification{
		Title: "Your match is ready",
		Body:  "Tap to take your seat.",
		URL:   "https://joinquest.cc/waiting",
		Tag:   "match-ready",
		TTL:   90 * time.Second,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotTTL != strconv.Itoa(90) {
		t.Fatalf("TTL header should carry the notification's TTL in seconds, got %q", gotTTL)
	}
	// The seat is being held with a deadline, so the push must not be batched.
	if gotUrgency != string(webpush.UrgencyHigh) {
		t.Fatalf("expected high urgency, got %q", gotUrgency)
	}
	// Payload must be encrypted end-to-end: the push service is untrusted.
	if gotEncoding != "aes128gcm" {
		t.Fatalf("expected an encrypted payload, got Content-Encoding %q", gotEncoding)
	}
}

func TestSendDefaultsTheTTLWhenTheCallerDoesNotSetOne(t *testing.T) {
	var gotTTL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTTL = r.Header.Get("TTL")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sender := newTestSender(t)
	if err := sender.Send(context.Background(), testSubscription(server.URL), Notification{
		Title: "Your match is ready",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotTTL != strconv.Itoa(int(DefaultTTL.Seconds())) {
		t.Fatalf("expected the default TTL, got %q", gotTTL)
	}
}

func TestSendReportsAPermanentlyRejectedSubscriptionAsGone(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()

			sender := newTestSender(t)
			err := sender.Send(context.Background(), testSubscription(server.URL), Notification{Title: "x"})
			if !errors.Is(err, ErrSubscriptionGone) {
				t.Fatalf("expected ErrSubscriptionGone so the dead endpoint is dropped, got %v", err)
			}
		})
	}
}

func TestSendKeepsTheSubscriptionWhenTheFailureIsTransient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	sender := newTestSender(t)
	err := sender.Send(context.Background(), testSubscription(server.URL), Notification{Title: "x"})
	if err == nil {
		t.Fatal("a 503 is still a failed send")
	}
	if errors.Is(err, ErrSubscriptionGone) {
		t.Fatal("a transient push service failure must not delete the player's subscription")
	}
}

func TestSendFailsClosedWhenPushIsNotConfigured(t *testing.T) {
	var unconfigured *WebPushSender
	if err := unconfigured.Send(context.Background(), testSubscription("https://push.example.com/x"), Notification{}); err == nil {
		t.Fatal("an unconfigured sender must report an error rather than silently dropping the notification")
	}

	empty := NewWebPushSender(Config{})
	if err := empty.Send(context.Background(), testSubscription("https://push.example.com/x"), Notification{}); err == nil {
		t.Fatal("a sender with no VAPID keys must report an error")
	}
}

func TestConfigFromEnvReportsPushUnconfiguredWithoutBothKeys(t *testing.T) {
	t.Setenv("VAPID_PUBLIC_KEY", "")
	t.Setenv("VAPID_PRIVATE_KEY", "")
	if _, ok := ConfigFromEnv(); ok {
		t.Fatal("push must not report itself configured with no keys")
	}

	t.Setenv("VAPID_PUBLIC_KEY", "public-only")
	if _, ok := ConfigFromEnv(); ok {
		t.Fatal("a public key alone cannot sign; push is not configured")
	}
}

func TestConfigFromEnvDefaultsTheVapidSubject(t *testing.T) {
	t.Setenv("VAPID_PUBLIC_KEY", "pub")
	t.Setenv("VAPID_PRIVATE_KEY", "priv")
	t.Setenv("VAPID_SUBJECT", "")

	cfg, ok := ConfigFromEnv()
	if !ok {
		t.Fatal("push should be configured when both keys are present")
	}
	// The VAPID spec requires a contact; an empty one is rejected by some
	// push services, so we must not send one.
	if cfg.Subject == "" {
		t.Fatal("a VAPID subject is required and must be defaulted")
	}
}

func TestLogSenderReportsPushUnavailableSoTheUIHidesTheAffordance(t *testing.T) {
	var sender Sender = LogSender{}
	if sender.PublicKey() != "" {
		t.Fatal("the log sender must report no public key, so the frontend treats push as unavailable")
	}
	if err := sender.Send(context.Background(), testSubscription("https://push.example.com/x"), Notification{Title: "x"}); err != nil {
		t.Fatalf("the log sender should never fail: %v", err)
	}
}
