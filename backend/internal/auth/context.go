package auth

import (
	"context"
	"net/http"
)

type contextKey string

const (
	userIDContextKey         contextKey = "authUserID"
	sessionTokenContextKey   contextKey = "authSessionToken"
	responseWriterContextKey contextKey = "authResponseWriter"
	userAgentContextKey      contextKey = "authUserAgent"
)

// WithUserID attaches an authenticated user ID to the context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// UserIDFromContext returns the authenticated user ID when present.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok && userID != ""
}

// WithSessionToken attaches the verified session JWT to the context.
func WithSessionToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, sessionTokenContextKey, token)
}

// SessionTokenFromContext returns the session JWT when present.
func SessionTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(sessionTokenContextKey).(string)
	return token, ok && token != ""
}

// WithResponseWriter attaches the HTTP response writer for cookie-setting resolvers.
func WithResponseWriter(ctx context.Context, w http.ResponseWriter) context.Context {
	return context.WithValue(ctx, responseWriterContextKey, w)
}

// ResponseWriterFromContext returns the response writer when available.
func ResponseWriterFromContext(ctx context.Context) (http.ResponseWriter, bool) {
	writer, ok := ctx.Value(responseWriterContextKey).(http.ResponseWriter)
	return writer, ok && writer != nil
}

// WithUserAgent attaches the request's User-Agent header.
func WithUserAgent(ctx context.Context, userAgent string) context.Context {
	return context.WithValue(ctx, userAgentContextKey, userAgent)
}

// UserAgentFromContext returns the request's User-Agent when present. Recorded
// against push subscriptions so the iOS/Android split can be read off real
// registrations rather than guessed at (JQ-198).
func UserAgentFromContext(ctx context.Context) string {
	userAgent, _ := ctx.Value(userAgentContextKey).(string)
	return userAgent
}
