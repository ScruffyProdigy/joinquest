/**
 * What the post-match return screen offers, and to whom.
 *
 * This lives outside the components because the prototype splits one decision across two
 * places: the roster is a card in the scrolling content, and the actions are a bar pinned to
 * the bottom of the screen. Both read the same viewer and the same result, so the reading is
 * here rather than duplicated on either side of that gap (JQ-277).
 */

/**
 * Finish reasons that mean the player left the match before it ended. They have been back in
 * the lobby since, so a board of who is still deciding is not what they returned for — they
 * get the actions and nothing else (JQ-233).
 *
 * `DISCONNECT` is deliberately absent: a drop is usually an accident, and taking the roster
 * away as well as the match is a second punishment for it. A null reason means the game never
 * reported this player at all, which is not evidence of anything — it keeps the roster too.
 *
 * The other early exit the prototype names — finishing first while the others play on — is
 * not readable from this data and does not need to be. `finishedAt` is stamped by whichever
 * of the game's report and the player's own landing on `/return` gets there first
 * (`MarkParticipantFinished`, `AND finished_at IS NULL`), so ordering exits by it races two
 * browsers against each other. That player is served by the screen's other state instead:
 * while the match is unfinished there is no roster to show.
 */
const EXITED_EARLY_REASONS = new Set(['ELIMINATED', 'FORFEIT'])

/**
 * The people the viewer came into the match with — their own group, not the six players the
 * match happened to contain (JQ-291).
 *
 * The server answers this per row, because "with whom" is a fact about arrival that only it
 * holds: everyone who queued from the same room table shares an arrival party, and a player
 * who joined the catalog queue alone shares one with nobody. A 3v3 between two groups puts
 * three names on each group's card, and the opposing three on neither.
 *
 * The viewer's own row is included, because the card marks it "You" and counts it. It is
 * absent only for a player who arrived alone, whose party is empty — and whose card
 * `showsRegroupRoster` does not render at all.
 */
export function arrivalPartyOf(result) {
  const participants = result?.participants ?? []
  return participants.filter((participant) => participant?.arrivalParty === true)
}

/** The viewer's own row, or null when they are not on this roster at all. */
export function viewerParticipantOf(result, viewerId) {
  if (!viewerId) {
    return null
  }
  const participants = result?.participants ?? []
  return participants.find((participant) => participant?.user?.id === viewerId) ?? null
}

export function hasExitedEarly(participant) {
  return EXITED_EARLY_REASONS.has(participant?.reason)
}

/**
 * Who the roster is for: a group who saw the match out. Both halves are load-bearing.
 *
 * A solo player has nobody to regroup with — they queued alone, and the people on that list
 * are strangers they never agreed to come back with. A group arrived through a room, by QR
 * code in the same place or by share link from different ones, and going back to that room
 * together is exactly what they expect. Confirmed with Ryan (JQ-233).
 *
 * And a player who exited early stopped being part of that decision when they left.
 *
 * `complete` is the third condition and it used to be implicit — the card was rendered inside
 * the screen's finished-match branch, so it could not appear early. Now that the card sits in
 * one flow with the standings it has to say so itself: before the match ends there is no
 * regroup to answer, and `playAgain` would be refused with SESSION_NOT_FINISHED.
 */
export function showsRegroupRoster(result, viewerId) {
  return (
    Boolean(result?.complete) &&
    result?.groupPlay === true &&
    !hasExitedEarly(viewerParticipantOf(result, viewerId))
  )
}

/**
 * Which of the ways out to offer. Note what is *not* here: nothing turns the primary action
 * off. `playAgain` is the only thing in the system that moves a participant to IN, and the
 * primary button is its only caller, so gating it on a quorum deadlocks the feature — two
 * players both landing here PENDING would each wait forever for the other (JQ-183).
 */
export function regroupActionsFor(result, viewerId) {
  // Already IN means the seat is claimed and the table exists: opting in again is not a thing
  // to ask for, so the same button becomes the way back to that table.
  const viewerIn = viewerParticipantOf(result, viewerId)?.regroup === 'IN'
  // A solo player's primary action replays the role and options they just had, so they are
  // the only ones who need a way to reach those choices again — a group was never given them
  // back (JQ-232).
  const showChooseAgain =
    result?.groupPlay === false && Boolean(result?.mode?.hasPreMatchChoice) && !viewerIn
  return {
    viewerIn,
    showChooseAgain,
    // Both lead back to the game, so only one is offered; the more specific promise wins.
    //
    // The `modes.length > 1` gate this used to carry is gone (JQ-233). It came from the
    // prototype, where it reads as "only offer the mode picker when there is a mode to pick",
    // but the destination is the game's page and not its picker. Keeping it meant that after
    // an RPSLR duel — one mode — the only ways off this screen were to wait for someone who
    // had already gone, or to leave for the catalog.
    showBackToGame: !showChooseAgain && Boolean(result?.game?.name),
  }
}
