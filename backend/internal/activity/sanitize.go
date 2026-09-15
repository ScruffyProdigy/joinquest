package activity

import "strings"

// Payload key tokens that may never be recorded.
//
// JQ-143 permits exactly one identifying value on an event -- the lobby user id, in
// its own column -- and forbids any other personal data in the payload. That is
// stated in the ticket as a rule, and a rule that lives only in a doc comment is one
// careless map literal away from being broken silently, years before anyone reads
// the table again. So it is enforced here instead.
//
// Matched as whole tokens, not substrings. Substring matching is the obvious
// implementation and it is wrong in a way that is nearly invisible: "participant"
// contains "ip", so a substring rule would silently strip participant_count -- a
// legitimate and useful signal -- and nobody would find out until an analysis came
// up short months later. Tokens are compared after splitting on separators and
// camelCase, so "ip" catches client_ip and ipAddress and leaves participant_count
// alone.
//
// Compound spellings that survive tokenisation as a single word (username,
// useragent) need their own entries; there is no way to find them by splitting.
var forbiddenPayloadKeyTokens = map[string]bool{
	"email":     true,
	"name":      true, // display_name, first_name, nickname via tokens below
	"username":  true,
	"nickname":  true,
	"ip":        true,
	"useragent": true,
	"agent":     true, // the "agent" half of user_agent
	"password":  true,
	"token":     true,
	"secret":    true,
	"address":   true,
	"phone":     true,
}

// sanitizePayload returns a copy with forbidden keys removed.
//
// Enforced at Record rather than checked in review, and by removing the key rather
// than rejecting the event: the rest of an event's payload is usually the part worth
// having, and discarding a whole signal because one field was careless would trade a
// privacy problem for a data problem. Returns nil for an empty payload so the column
// keeps its '{}' default rather than storing an empty object per row.
func sanitizePayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}

	cleaned := make(map[string]any, len(payload))
	for key, value := range payload {
		if forbiddenPayloadKey(key) {
			continue
		}
		cleaned[key] = value
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

// forbiddenPayloadKey reports whether a payload key names personal data.
func forbiddenPayloadKey(key string) bool {
	for _, token := range payloadKeyTokens(key) {
		if forbiddenPayloadKeyTokens[token] {
			return true
		}
	}
	return false
}

// payloadKeyTokens splits a payload key into lowercased words, treating both
// separators and camelCase humps as boundaries so that snake_case, kebab-case and
// camelCase keys all reduce to the same tokens.
func payloadKeyTokens(key string) []string {
	var (
		tokens  []string
		current strings.Builder
	)
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
	}

	runes := []rune(key)
	for i, r := range runes {
		switch {
		case isASCIILetter(r) || isASCIIDigit(r):
			// A capital starting a new word ends the previous one, so "ipAddress"
			// splits but "ID" stays whole.
			if isASCIIUpper(r) && i > 0 && !isASCIIUpper(runes[i-1]) {
				flush()
			}
			current.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return tokens
}

func isASCIILetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func isASCIIDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isASCIIUpper(r rune) bool  { return r >= 'A' && r <= 'Z' }
