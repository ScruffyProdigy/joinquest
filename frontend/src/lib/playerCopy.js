/** Player-facing strings (avoid “queue” in the shell UI). */

export const APP_TAGLINE = 'Solo or squad, just join.'

/** Home page: the greeting row, and the label the catalog gives itself. */
export const GREETING_MORNING = 'Good morning'
export const GREETING_AFTERNOON = 'Good afternoon'
export const GREETING_EVENING = 'Good evening'
export const ALL_GAMES_HEADING = 'All games'
export const SIGN_IN_OR_JOIN = 'Sign in or Join'
/** Chip label for a signed-in player who has not picked a name yet. */
export const ACCOUNT_CHIP_FALLBACK = 'Account'
export const SIGN_IN_DIALOG_TITLE = 'Sign in or create an account'

/**
 * The prototype makes the case for an account with four rotating cards rather
 * than one static line, so the reasons are a list here, not a sentence.
 *
 * House style is en-GB ("favourites"), which is both the prototype's own
 * spelling and the spelling used throughout this repo's prose. It is the only
 * -our/-or word in player-facing copy, so the whole convention rests here.
 */
export const SIGN_IN_BENEFITS = [
  {
    key: 'handle',
    headline: 'Own your handle.',
    body: 'Pick exactly what you want to be called — not something the randomizer handed you.',
  },
  {
    key: 'history',
    headline: 'Your history travels with you.',
    body: 'Every win, every session — all in one place, across every device you play on.',
  },
  {
    key: 'crew',
    headline: 'Your people, always ready.',
    body: 'Save your crew and jump back into a game without hunting them down first.',
  },
  {
    key: 'preferences',
    headline: 'Never re-enter your preferences.',
    body: 'Your roles, settings, and favourites stay saved between every session.',
  },
]

/**
 * The prototype pairs the cards with "Create an account" / "Maybe later". Here the
 * sign-in panel below the cards *is* the create-an-account affordance, so only the
 * dismissal needs a button of its own.
 */
export const SIGN_IN_MAYBE_LATER = 'Maybe later'
/** Names one dot for a screen reader, e.g. "Reason 2 of 4". */
export function signInBenefitDotLabel(index, total) {
  return `Reason ${index + 1} of ${total}`
}
export const SIGN_IN_BENEFITS_LABEL = 'Reasons to create an account'

/** Catalog: the developer pitch that sits in the game list as its own card. */
export const DEVELOPER_PROMO_TITLE = 'Your game could live here'
/** Same card, retitled when a search or filter leaves nothing to show. */
export const DEVELOPER_PROMO_TITLE_NO_RESULTS = 'Don’t see your game? Build it.'
export const DEVELOPER_PROMO_BODY = 'List it on JoinQuest via the developer portal'
export const DEVELOPER_PROMO_CTA = 'Building a game? Get started'

export const SIGN_IN_HEADING = 'Get in the game'
export const PLAY_AS_GUEST = 'Play as guest'
export const PLAY_AS_GUEST_HINT = 'Play now, make your account later.'
export const SIGN_IN_DIVIDER = 'Or'
export const SIGN_IN_DIVIDER_LABEL = 'Or sign in with email or social'
export const ACCOUNT_LINK_LABEL = 'Account settings'
export const GUEST_BADGE = 'Guest'
export const GUEST_SPIRIT_ANIMAL_HINT =
  'Nice find. Link an email in Account settings if you want this avatar on your next visit.'
/** First-entry avatar picker. Copy stays account-wide — it is not per-game. */
export const IDENTITY_GATE_HEADING = 'Welcome to JoinQuest'
export const IDENTITY_GATE_TAGLINE = 'Find and play your next game'
export const IDENTITY_GATE_PROMPT = 'Select an avatar to start playing'
export const IDENTITY_GATE_SCOPE_HINT =
  'This is your name and avatar across all of JoinQuest, not just one game.'
export const IDENTITY_GATE_DIVIDER = 'or'
export const IDENTITY_GATE_SIGN_IN = 'Log in or create account'
export const IDENTITY_GATE_ERROR = 'Could not start your session. Pick an avatar to try again.'
export const IDENTITY_GATE_BACK_TO_AVATARS = 'Back to avatars'
export const IDENTITY_GATE_BACK = 'Back'

/**
 * Half an identity. Someone signed in whose name or avatar is missing is not a
 * guest and must not be greeted as one — they are asked for the missing half
 * only, and told the half they already have is staying.
 */
export const NAME_PROMPT_HEADING = 'One more thing'
export const NAME_PROMPT_TAGLINE = 'We have your avatar — we just need a name to go with it.'
export const NAME_PROMPT_KEEPING_AVATAR = 'Your avatar stays as it is.'
export const NAME_PROMPT_LABEL = 'What should we call you?'
export const NAME_PROMPT_PLACEHOLDER = 'Your name'
export const NAME_PROMPT_SCOPE_HINT =
  'This is your name across all of JoinQuest, not just one game.'
export const NAME_PROMPT_SUBMIT = 'Save and continue'
export const NAME_PROMPT_SAVING = 'Saving…'
export const NAME_PROMPT_ERROR = 'Could not save your name. Try again.'

export const AVATAR_PROMPT_HEADING = 'Pick your face'
export const AVATAR_PROMPT_TAGLINE = 'We have your name — we just need a face to go with it.'
export const AVATAR_PROMPT_SCOPE_HINT =
  'This is your avatar across all of JoinQuest, not just one game.'
export const AVATAR_PROMPT_ERROR = 'Could not save your avatar. Pick one to try again.'

/** Names the player already has, so the picker cannot read as a fresh start. */
export function playingAsLine(name) {
  return `Playing as ${name}.`
}

export const GUEST_ACCOUNT_PROMPT =
  'You’re playing as a guest. Add an email in Account settings to keep your progress across visits.'

export function formatMergeWarning(sourceDisplayName, currentDisplayName) {
  const source = sourceDisplayName?.trim() || 'another account'
  const current = currentDisplayName?.trim() || 'your current account'
  return `This email belongs to ${source}. Linking it will move that account’s progress into ${current} and deactivate the other account.`
}

export const MERGE_CONFIRM = 'Yes, merge accounts'
export const MERGE_CANCEL = 'Cancel'

/**
 * The prototype's primary CTA. It used to name the guest-entry action, which is
 * why that one is now PLAY_AS_GUEST -- two buttons with this label and unrelated
 * behaviour would be worse than either wording on its own (JQ-249).
 *
 * "Look for group" survives as domain vocabulary (lib/tables.js backfill, the
 * banner lines below); only the button verb changed.
 */
export const JUMP_IN = 'Jump in'
export const STOP_FINDING = 'Stop'

export const PLAY_SOLO = 'Single player'
export const STARTING_SOLO = 'Starting…'

export function joinAsLabel(queuePath) {
  return `Join as ${queuePath}`
}

export function waitingAsRoleLine(queuePath) {
  return `Finding players as ${queuePath}…`
}

export const LAUNCH_GAME = 'Launch Now'
// The way back in after a closed tab, a dropped connection, or a phone call (JQ-86).
// A player standing in the lobby with a live match is returning to it, not starting it.
export const REJOIN_MATCH = 'Rejoin'
export const REJOIN_FAILED = 'Could not get you back in. Try again in a moment.'
export const REJOIN_OVER =
  'That match has finished, so there is nothing to rejoin.'
export const LEAVE_GAME = 'Leave game'
export const LEAVE_MATCH = 'Leave match'
export const LEAVE_GAME_FAILED = 'Could not leave right now. Please try again.'
export const LEAVE_GAME_NOT_FOUND =
  'We could not find that game to leave. Reload the page if this banner stays.'


export const OPTIONS_UNAVAILABLE =
  "This game can't tell us your options right now. Try again in a moment."

/** Names what the player has picked so far, for the picker footer. */
export function optionsChosenCount(groups, picks) {
  const chosen = groups.flatMap((group) => picks[group.key] ?? [])
  if (chosen.length === 0) {
    return 'Nothing chosen yet'
  }
  return `${chosen.length} chosen`
}

/** Flattens a player's stored picks into the labels shown beside their role. */
export function selectedOptionLabels(selectedOptions) {
  return (selectedOptions ?? []).flatMap((selection) => selection.labels ?? [])
}

export const GAMES_SEARCH_LABEL = 'Search games'
export const GAMES_SEARCH_PLACEHOLDER = 'Search games…'
export const GAMES_SEARCH_EMPTY = 'No games found'
export const GAMES_SEARCH_EMPTY_HINT = 'Try a different search term'

export function waitingForGroupLine(count) {
  const n = count ?? 0
  return `Looking for players… (${n} ${n === 1 ? 'player' : 'players'} looking)`
}

/**
 * How much longer this player is likely to wait, or null when the API declined
 * to say. Null is the API's honest answer whenever throughput is not measurable
 * and there is no usable history, or when the number came out too large to be
 * useful — so it renders as no line at all rather than as a hedge.
 *
 * Rounds to minutes once past one, because a live estimate that ticks between
 * "97 sec" and "104 sec" reads as noise rather than information.
 */
export function estimatedWaitLine(seconds) {
  if (seconds === null || seconds === undefined) {
    return null
  }
  if (seconds < 60) {
    // A queue about to pop should not read as "0 sec left".
    return `About ${Math.max(1, Math.round(seconds))} sec left`
  }
  return `About ${Math.round(seconds / 60)} min left`
}

export function bannerWaitingLine(
  gameName,
  count,
  queuePathDisplayName,
  formingGaps,
  selectedOptions,
) {
  const n = count ?? 0
  const players = `${n} ${n === 1 ? 'player' : 'players'} looking`
  const role = queuePathDisplayName?.trim()
  let line
  if (role) {
    line = `Looking for a group in ${gameName} as ${role} · ${players}`
  } else {
    line = `Looking for a group in ${gameName} · ${players}`
  }
  // The picks sit next to the role because that is the pair a waiting player
  // wants to confirm: which cohort am I in, and what did I bring.
  const picks = selectedOptionLabels(selectedOptions)
  if (picks.length > 0) {
    line += ` · ${picks.join(', ')}`
  }
  const needLine = formatFormingGapsNeedLine(formingGaps)
  if (needLine) {
    line += ` · ${needLine}`
  }
  return line
}

export function bannerIntentPlayingLine(gameName, modeName, roleLabel) {
  const mode = modeName?.trim()
  const role = roleLabel?.trim()
  if (mode && role) {
    return `Playing ${gameName} (${mode}) as ${role}`
  }
  if (mode) {
    return `Playing ${gameName} (${mode})`
  }
  return `Playing ${gameName}`
}

export function bannerIntentPlayingHint() {
  return 'Your seat is held — rejoin whenever you are ready.'
}

export function bannerIntentLaunchPendingHint() {
  return 'Preparing your launch link…'
}

/** Names the waiting page's card for assistive tech. */
export const WAITING_REGION_LABEL = 'Finding players'

/** The whole wait, wherever it shows: in-flight button, status line, waiting page. */
export const FINDING_PLAYERS = 'Finding players…'

// The launch moment (JQ-136): the match forming is an event, not a changed banner.
export const MATCH_FOUND = "You're in!"
export const READY_TO_LAUNCH = 'Ready to launch!'
export const LAUNCH_AUTO_HINT = 'Taking you into the game — no need to click.'
export const LAUNCH_HELD_HINT = 'Countdown paused while you were away. Launch when you are ready.'

export function launchCountdownLine(seconds) {
  return `Entering in ${Math.max(0, seconds ?? 0)}…`
}

/** e.g. "Word Hunt · Arena · Clue Giver · Hard mode" */
export function waitingPageSubline(gameName, modeName, roleLabel, selectedOptions) {
  const parts = [gameName, modeName, roleLabel].map((part) => part?.trim()).filter(Boolean)
  return [...parts, ...selectedOptionLabels(selectedOptions)].join(' · ')
}

// Leaving gives up the player's place, so it is confirmed first.
/* Notifications: leaving the page without losing your place. */

export const NOTIFY_ME = 'Notify me instead'
export const NOTIFY_ME_VALUE = 'Leave this page and keep your spot.'
export const NOTIFY_ME_IOS = 'Get notified on your phone'
export const NOTIFY_ON = "You'll be notified"
export const NOTIFY_ON_VALUE = 'Close this page whenever you like.'
export const NOTIFY_OFF = 'Turn off notifications'
export const NOTIFY_BLOCKED =
  'Notifications are switched off for JoinQuest in your browser settings.'
export const NOTIFY_FAILED = 'Notifications could not be switched on. Try again.'

export const INSTALL_TITLE = 'Add JoinQuest to your Home Screen'
export const INSTALL_BODY =
  'Safari only sends notifications from an installed app. It takes two taps, and your spot is held while you do it.'
export const INSTALL_STEP_SHARE = 'Tap Share'
export const INSTALL_STEP_SHARE_WHERE = 'in the bar at the bottom of the screen'
export const INSTALL_STEP_ADD = 'Choose Add to Home Screen'
export const INSTALL_STEP_ADD_WHERE = 'then open JoinQuest from your Home Screen'
export const INSTALL_DISMISS = 'Not now'

export const LEAVE_QUEUE_TITLE = 'Leave the queue?'
export const LEAVE_QUEUE_BODY = "You'll lose your spot and have to start over."
export const LEAVE_QUEUE_CONFIRM = 'Leave queue'
export const STAY_IN_QUEUE = 'Stay in queue'

export function bannerIntentWaitingHint() {
  return 'We will notify you here when your group is ready.'
}

export function bannerLiveUpdatesPausedHint() {
  return 'Live updates paused — refreshing every few seconds.'
}

export const PLAY_WITH_FRIENDS = 'Play with friends'
export const START_GAME = 'Start now'
export const DISCARD = 'Discard'
export const KING_LABEL = 'King'
export const LEAVE_TABLE_SEAT = 'Leave seat'

export function bannerTableSeatHint() {
  return 'Use Room below to manage seats or start the game.'
}

export function bannerTableBackfillHint(formingGaps) {
  const needLine = formatFormingGapsNeedLine(formingGaps)
  if (needLine) {
    return `Your table is queued to start — ${needLine.charAt(0).toLowerCase()}${needLine.slice(1)}`
  }
  return 'Your table is queued to start — waiting for players from the lobby.'
}

/** e.g. "Need 1 Clue Giver, 3 Guessers" */
export function formatFormingGapsNeedLine(formingGaps) {
  const parts = formatFormingGapParts(formingGaps)
  if (parts.length === 0) {
    return null
  }
  return `Need ${parts.join(', ')}`
}

/** e.g. "Need 1 Clue Giver, 3 Guessers from the lobby" */
export function formatFormingGapsFromLobbyLine(formingGaps) {
  const parts = formatFormingGapParts(formingGaps)
  if (parts.length === 0) {
    return null
  }
  return `Need ${parts.join(', ')} from the lobby`
}

function formatFormingGapParts(formingGaps) {
  if (!Array.isArray(formingGaps)) {
    return []
  }
  return formingGaps
    .filter((gap) => (gap?.needed ?? 0) > 0)
    .map((gap) => {
      const count = gap.needed
      const label = gap.displayName?.trim() || gap.queuePath?.trim() || 'player'
      return `${count} ${label}`
    })
}

export function bannerTableStartedLine(gameName, modeName) {
  return `Your game is ready — ${gameName} (${modeName})`
}

export function bannerTableStartedHint() {
  return 'Launch when you are ready — your seat is reserved.'
}

export function bannerTableSeatLine(gameName, modeName, seatDisplayName) {
  const role = seatDisplayName?.trim()
  if (role) {
    return `Seated at ${gameName} (${modeName}) as ${role}`
  }
  return `Seated at ${gameName} (${modeName})`
}

export function switchedFromGroupMessage(gameName) {
  return `You left the group for ${gameName} to look for a group here.`
}

/** Post-game return: standings, regroup, and the actions that leave it. */
export const RESULTS_IN_PROGRESS_TITLE = 'Results so far'
export const RESULTS_FINAL_TITLE = 'Final standings'
export const RESULTS_WINNER = 'Winner'
export const RESULTS_STILL_PLAYING = 'Still playing'
export const RESULTS_IN_PROGRESS = 'In progress…'
/** Screen-reader text for the pulsing placement dot — only when it isn't already covered by "Still playing". */
export const RESULTS_PLACEMENT_UNKNOWN = 'Placement not yet known'
export const RESULTS_YOU = 'You'

export const REGROUP_TITLE = 'Who’s playing again?'
export const REGROUP_IN = 'In'
export const REGROUP_OUT = 'Out'
export const REGROUP_PENDING = 'Not back yet'
export const REGROUP_ANOTHER_ROUND = 'Another round'
/** Shown instead of "Another round" once you are already in: the seat is claimed, so go sit in it. */
export const REGROUP_BACK_TO_TABLE = 'Back to the table'
export const REGROUP_FIND_SOMETHING_NEW = 'Find something new'
// A solo player's primary action replays the role and options they just had, so this is
// how they reach those choices instead. Absent, not disabled, when there is nothing to
// choose (JQ-232).
export const REGROUP_CHOOSE_AGAIN = 'Choose again'

// Confirms the pre-queue picker when it is claiming a seat rather than joining a queue.
export const GROUP_TAKE_SEAT = 'Take this seat'
export const GROUP_TAKING_SEAT = 'Taking your seat…'
// The way off the group screen that is not "leave": the rejoin surface is a destination.
export const GROUP_FIND_SOMETHING_NEW = 'Find something new'

export function formatBackToGame(gameName) {
  return `Back to ${gameName}`
}

/**
 * Informational, never a gate. "Another round" is the only thing in the system that moves
 * a player to IN, so this line may never be allowed to disable it — it exists so a player
 * can see whether anyone else is coming back, and what the mode needs to fill a table.
 * Starting the next match is the table's job, not this screen's.
 */
export function formatRegroupInCount(inCount, total, minPlayers) {
  const roster = `${inCount} of ${total} back and in`
  return Number.isFinite(minPlayers) && minPlayers > 0 ? `${roster} · needs ${minPlayers} to start` : roster
}

/** The still-playing branch's exit, so waiting on a live match is never a dead end. */
export const RESULTS_LEAVE_MATCH = 'Head back to JoinQuest'
/** Live updates failed to connect: the screen still shows what it fetched, but it will not move. */
export const RESULTS_LIVE_UPDATES_OFF = 'Live updates are off. Reload to see the latest.'

/**
 * Regroup failures the player can act on. `ErrNoRegroupMode` has no message: it is a
 * documented degradation, so the client routes to the game's detail page instead of
 * telling the player about a mode that no longer exists.
 */
export const REGROUP_ERROR_NOT_FINISHED = 'That match hasn’t finished yet.'
export const REGROUP_ERROR_TABLE_FULL = 'The table filled up before you got a seat.'
export const REGROUP_ERROR_GENERIC = 'Could not start another round. Try again.'

/** Ordinal for a placement: 1 → "1st", 2 → "2nd", 11 → "11th". */
export function ordinal(n) {
  const rem100 = n % 100
  if (rem100 >= 11 && rem100 <= 13) return `${n}th`
  switch (n % 10) {
    case 1: return `${n}st`
    case 2: return `${n}nd`
    case 3: return `${n}rd`
    default: return `${n}th`
  }
}

/**
 * Headline and sub for the return screen, from the match outcome.
 *
 * `complete` is `MatchResult.complete` — whether the *match* has finished,
 * not just this participant's own play. A participant can have
 * `reason: 'COMPLETED'` (they finished their own play) while the match is
 * still running for others; that state has no `PlayerFinishReason` value of
 * its own, so it's expressed here via `complete` rather than `reason`.
 *
 * `reason === 'ELIMINATED'` is checked first, ahead of `complete`, because
 * elimination is real and worth calling out even while the match is still
 * running for everyone else — "Eliminated before the end." is more specific
 * than the generic "Waiting for others to finish." and shouldn't be masked
 * by it.
 */
/**
 * How the result feels, which is what the mascot panel above the headline is tinted by.
 * The prototype's `rd()` produces exactly these three — it has a fourth, `waiting`, that
 * nothing on this screen can reach, so it is not carried over.
 */
export const MATCH_MOOD = Object.freeze({ WIN: 'win', LOSE: 'lose', CLOSE: 'close' })

/** The prototype ships the mascot as a labelled placeholder; the label is the copy. */
export const MASCOT_LABEL = Object.freeze({
  [MATCH_MOOD.WIN]: 'Mascot illustration · celebrating',
  [MATCH_MOOD.LOSE]: 'Mascot illustration · sympathetic',
  [MATCH_MOOD.CLOSE]: 'Mascot illustration · tense',
})

export function matchHeadline({ reason, complete, placement, playerCount }) {
  // `placement` is nullable on MatchParticipantResult — the game may report a finish with no
  // placement, or not have reported this player at all — and ordinal(null) is "nullth". Such
  // a player keeps the sub that describes their situation and simply loses the number.
  if (placement == null) {
    if (reason === 'ELIMINATED') {
      return { headline: RESULTS_PLACEMENT_UNKNOWN, sub: 'Eliminated before the end.', mood: MATCH_MOOD.LOSE }
    }
    if (!complete) {
      return { headline: RESULTS_PLACEMENT_UNKNOWN, sub: 'Waiting for others to finish.', mood: MATCH_MOOD.CLOSE }
    }
    return { headline: RESULTS_PLACEMENT_UNKNOWN, sub: `Out of ${playerCount} players.`, mood: MATCH_MOOD.CLOSE }
  }
  if (reason === 'ELIMINATED') {
    return {
      headline: `${ordinal(placement)} of ${playerCount}.`,
      sub: 'Eliminated before the end.',
      mood: MATCH_MOOD.LOSE,
    }
  }
  if (!complete) {
    return {
      headline: `${ordinal(placement)} place.`,
      sub: 'Waiting for others to finish.',
      // Out in front while the others play on is the one waiting state that is good news.
      mood: placement === 1 ? MATCH_MOOD.WIN : MATCH_MOOD.CLOSE,
    }
  }
  if (placement === 1) {
    return { headline: '1st place.', sub: 'You finished on top.', mood: MATCH_MOOD.WIN }
  }
  if (placement === playerCount) {
    return { headline: `${ordinal(placement)} place.`, sub: 'Better luck next time.', mood: MATCH_MOOD.LOSE }
  }
  return {
    headline: `${ordinal(placement)} place.`,
    sub: `Out of ${playerCount} players.`,
    mood: MATCH_MOOD.CLOSE,
  }
}
