package rating

import "strings"

// Entrant key grammar.
//
// Every entrant on a side carries one of these shapes, and the prefix is what
// decides which table the rating lands in and how the identifiability report
// reads it:
//
//	player:<uuid>                     a player's mode-level skill (a main effect)
//	player:<uuid>@seat:<NamePath>     that player's skill in one seat (an interaction)
//	seat:<NamePath>                   the seat class itself (a main effect)
//	scenario:<key>                    a cooperative scenario
//	prequeue:<group>/<option>         a rated pre-queue selection
//
// The first two live in player_ratings, keyed by (user_id, game_id, mode_key,
// seat_class) with seat_class empty for the mode-level row. Everything else
// lives in nonplayer_ratings.
//
// The interaction key deliberately embeds the plain player key as a prefix.
// carrySourceRatingHistoryTx (internal/store/user_merge_carry.go) rewrites a
// merged-away user's history with a text REPLACE of "player:<uuid>", so a
// player's per-role history follows them through a merge with no extra
// machinery — see the comment there, which depends on this property.
const (
	playerKeyPrefix = "player:"
	seatKeyPrefix   = "seat:"

	// seatInteractionSeparator joins a player key to the seat class they held.
	// "@" is safe: a UUID cannot contain it, and a seat NamePath joins its
	// segments with "/" (see internal/seattemplate), so neither side of the
	// separator can produce a false split.
	seatInteractionSeparator = "@seat:"
)

// PlayerKey is the entrant key for a player's mode-level rating.
func PlayerKey(playerID string) string {
	return playerKeyPrefix + playerID
}

// PlayerSeatKey is the entrant key for a player's rating in one seat class —
// the interaction between the two. An empty seatClass yields the mode-level
// key, so callers do not have to branch on whether a mode is asymmetric.
func PlayerSeatKey(playerID, seatClass string) string {
	if seatClass == "" {
		return PlayerKey(playerID)
	}
	return PlayerKey(playerID) + seatInteractionSeparator + seatClass
}

// SeatKey is the entrant key for a seat class's own rating — the main effect
// JQ-139 introduced, pooled across every player who has held the seat.
func SeatKey(seatClass string) string {
	return seatKeyPrefix + seatClass
}

// ParsePlayerKey splits a player entrant key into the player id and the seat
// class it is scoped to. seatClass is empty for a mode-level key. ok is false
// for any key that does not name a player at all (a seat class, a scenario, a
// pre-queue option), which is what routes an entrant to nonplayer_ratings.
func ParsePlayerKey(key string) (playerID, seatClass string, ok bool) {
	rest, ok := strings.CutPrefix(key, playerKeyPrefix)
	if !ok {
		return "", "", false
	}
	if id, class, found := strings.Cut(rest, seatInteractionSeparator); found {
		return id, class, true
	}
	return rest, "", true
}

// IsPlayerKey reports whether an entrant key names a player, at either
// granularity.
func IsPlayerKey(key string) bool {
	_, _, ok := ParsePlayerKey(key)
	return ok
}
