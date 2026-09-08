package developer

import "fmt"

// CatalogOption describes one allowed value on a catalog axis.
type CatalogOption struct {
	ID          string
	Label       string
	Description string
}

// CatalogTag is the legacy name for a catalog axis value.
//
// Deprecated: the flat tag list was replaced by genre, difficulty and social
// mode in JQ-162. Kept so the deprecated catalogTagTaxonomy query still
// compiles; remove with LegacyCatalogTagTaxonomy.
type CatalogTag = CatalogOption

// GenreTaxonomy is the game-level genre vocabulary: what the game is *about*.
//
// Exactly one per game, because the card shows one label and a game that claims
// two genres claims neither. The six are the prototype's; adding a genre later
// is cheap, removing one strands the games that chose it.
var GenreTaxonomy = []CatalogOption{
	{ID: "action", Label: "Action", Description: "Reflexes, timing, real-time pressure"},
	{ID: "strategy", Label: "Strategy", Description: "Planning, tactics, long-run decisions"},
	{ID: "deduction", Label: "Deduction", Description: "Hidden information, reading other players"},
	{ID: "words-trivia", Label: "Words & Trivia", Description: "Language, knowledge, recall"},
	{ID: "drawing-creative", Label: "Drawing & Creative", Description: "Making something others judge"},
	{ID: "puzzle", Label: "Puzzle", Description: "Solving a defined problem"},
}

// DifficultyTaxonomy is the game-level difficulty floor: how much a new player
// must know before their first round is any fun.
//
// This is the developer's own declaration, not a measurement. It is the signal
// the old `casual` tag carried, promoted out of the tag list so JQ-145 can use
// it as a ground-truth cross-check instead of digging it back out.
var DifficultyTaxonomy = []CatalogOption{
	{ID: "casual", Label: "Casual", Description: "Low pressure, playable without instructions"},
	{ID: "involved", Label: "Involved", Description: "Takes a round or two before it clicks"},
	{ID: "demanding", Label: "Demanding", Description: "Expects the rules known up front"},
}

// SocialModeTaxonomy is the *mode*-level social shape: how play is structured.
//
// It sits on the mode rather than the game because a game's modes genuinely
// disagree — Word Hunt's Arena is free-for-all and its Duel is 1v1 — and a
// game-level tag cannot say that.
var SocialModeTaxonomy = []CatalogOption{
	{ID: "free-for-all", Label: "Free-for-all", Description: "Everyone plays for themselves"},
	{ID: "1v1", Label: "1v1", Description: "Exactly two players head-to-head"},
	{ID: "teams", Label: "Teams", Description: "Players win or lose as a side"},
	{ID: "hidden-roles", Label: "Hidden roles", Description: "Players hold secret allegiances"},
	{ID: "co-op", Label: "Co-op", Description: "Everyone wins or loses together"},
}

// LegacyCatalogTagTaxonomy is the retired flat tag vocabulary.
//
// Deprecated: served only by the deprecated catalogTagTaxonomy query so a client
// built before JQ-162 can still label existing games' tags during a rolling
// deploy. Nothing writes tags any more. Remove one release after JQ-162 ships.
var LegacyCatalogTagTaxonomy = []CatalogOption{
	{ID: "competitive", Label: "Competitive", Description: "Winners and losers, direct opposition"},
	{ID: "cooperative", Label: "Co-op", Description: "Players win or lose together"},
	{ID: "party", Label: "Party", Description: "Social groups, casual fun"},
	{ID: "1v1", Label: "1v1", Description: "Exactly two players head-to-head"},
	{ID: "quick", Label: "Quick", Description: "Sessions under about ten minutes"},
	{ID: "words", Label: "Words", Description: "Word and language puzzles"},
	{ID: "strategy", Label: "Strategy", Description: "Planning, hidden info, tactics"},
	{ID: "casual", Label: "Casual", Description: "Low pressure, easy to pick up"},
}

func validateOption(axis string, options []CatalogOption, value string) error {
	if value == "" {
		return nil
	}
	for _, o := range options {
		if o.ID == value {
			return nil
		}
	}
	return fmt.Errorf("developer: unknown %s %q", axis, value)
}

// ValidateGenre accepts a genre ID or the empty string (genre not set).
func ValidateGenre(genre string) error {
	return validateOption("genre", GenreTaxonomy, genre)
}

// ValidateDifficulty accepts a difficulty ID or the empty string (not declared).
func ValidateDifficulty(difficulty string) error {
	return validateOption("difficulty", DifficultyTaxonomy, difficulty)
}

// ValidateSocialMode accepts a social mode ID or the empty string (not declared).
func ValidateSocialMode(socialMode string) error {
	return validateOption("social mode", SocialModeTaxonomy, socialMode)
}
