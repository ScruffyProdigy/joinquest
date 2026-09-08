package developer

// DiscoveryPrompt returns the agent interview script for understanding a game
// before drafting catalog metadata or seatTemplate guidance.
func DiscoveryPrompt() string {
	return `# Discover your game

Start with one open-ended prompt — do not lead with a checklist.

**Opening (ask first):**

> Tell me about the game you're thinking of — as much or as little as you have. What's the idea, how do people play together, who it's for, anything you're excited about or unsure about.

**Then clarify only what's missing:**

Read what they shared and identify gaps. You need enough to draft catalog copy and a seatTemplate plan. Ask follow-ups conversationally — one or two at a time, not a wall of questions. If they already answered something, do not re-ask it.

**Confirm what you think you know:** If you can infer an answer but aren't fully sure, check it with the developer instead of guessing or asking from scratch. For example: "It sounds like this is mostly a 2-player game — would you say that's fair?" Same for structure, vibe, session length, and the rest.

| Topic | Why it matters | Only ask if unclear |
|-------|----------------|---------------------|
| Player count | seatTemplate / game-modes | min/max, fixed or variable |
| Structure | seatTemplate | duel, free-for-all, teams, or roles |
| Genre | genre | action, strategy, deduction, words & trivia, drawing & creative, or puzzle |
| Social mode | socialMode on each mode | free-for-all, 1v1, teams, hidden roles, or co-op — ask per mode, they often differ |
| Difficulty | difficulty | how much a new player must know before their first round is fun |
| Vibe / audience | catalog voice | brainy, chaotic, tactical, etc. — copy, not a field |
| API URL | registration | public HTTPS hosting plan (not localhost) |

**Draft (show for approval, do not save yet):**
- shortDescription (~120 chars, JoinQuest tone)
- longDescription (2–4 paragraphs for the detail page)
- howToPlay (3–6 bullet steps)
- genre (exactly one ID from genreTaxonomy) and difficulty (optional, from difficultyTaxonomy)
- socialMode per mode, declared in the game's /api/v1/game-modes rather than in metadata
- seatTemplate guidance (point to seat-templates cookbook: duel, 3v3, composition)

Always show drafts to the developer for approval before calling updateMyGameMetadata.
`
}
