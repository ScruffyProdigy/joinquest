import {
  gameDifficultyLabel,
  gameGenreLabel,
  modeSocialModeLabel,
} from "./gameCard";
import { graphqlRequest } from "./graphql";

const GAME_MODE_FIELDS = `
  modes {
    id
    modeKey
    displayName
    status
    minPlayers
    maxPlayers
    typicalMinutes
    queuePaths {
      queuePath
      displayName
      playersToStart
    }
    seats {
      queuePath
    }
    queues {
      id
      name
      playersToStart
      status
    }
    preQueueGroups {
      key
      kind
      label
      min
      max
    }
    queueOptions(playerId: $playerId) @include(if: $hasPlayer) {
      available
      unavailableReason
      groups {
        key
        choices {
          id
          label
          description
          locked
          unlockModeKey
          requirement {
            __typename
            label
            ... on RequirementLeaf {
              current
              target
            }
            ... on RequirementGroup {
              operator
              children {
                __typename
                label
                ... on RequirementLeaf {
                  current
                  target
                }
              }
            }
          }
        }
      }
    }
    eligibility(playerId: $playerId) @include(if: $hasPlayer) {
      accessible
      reason
      unlockModeKey
      requirement {
        __typename
        label
        ... on RequirementLeaf {
          current
          target
        }
        ... on RequirementGroup {
          operator
          children {
            __typename
            label
            ... on RequirementLeaf {
              current
              target
            }
          }
        }
      }
    }
  }
`;

const GAME_CARD_FIELDS = `
  id
  slug
  name
  iconUrl
  heroUrl
  catalogHeroUrl
  titleArt {
    url
    anchor
    widthPct
  }
  shortDescription
  longDescription
  howToPlay
  tutorialUrl
  screenshots
  genre
  difficulty
  accentColor
  playerActivity {
    playing
    queued
  }
  createdAt
  ${GAME_MODE_FIELDS}
`;

const GAMES_QUERY = `
  query Games($playerId: ID!, $hasPlayer: Boolean!) {
    games {
      ${GAME_CARD_FIELDS}
    }
  }
`;

const GAME_BY_SLUG_QUERY = `
  query GameBySlug($slug: String!, $playerId: ID!, $hasPlayer: Boolean!) {
    gameBySlug(slug: $slug) {
      ${GAME_CARD_FIELDS}
    }
  }
`;

/** Parse /games/:slug from a pathname. */
export function parseGameSlug(pathname) {
  const match = String(pathname || "").match(/^\/games\/([^/]+)\/?$/);
  return match?.[1] ? decodeURIComponent(match[1]) : null;
}

/** Pick the first active queue for a game (typically the default queue). */
export function defaultQueueForGame(game) {
  for (const mode of game?.modes ?? []) {
    for (const queue of mode?.queues ?? []) {
      if (queue.status === "active") {
        return queue;
      }
    }
  }
  return null;
}

/** Mode that owns the default active queue for this game. */
export function defaultModeForGame(game) {
  for (const mode of game?.modes ?? []) {
    for (const queue of mode?.queues ?? []) {
      if (queue.status === "active") {
        return mode;
      }
    }
  }
  return null;
}

/** A single-seat mode (e.g. a solo tutorial) fires instantly for the one joining player. */
export function isSoloMode(mode) {
  return mode?.minPlayers === 1 && mode?.maxPlayers === 1;
}

/** Player-count badge label for a mode, or null when the range isn't known. */
export function modePlayerRangeLabel(mode) {
  const min = mode?.minPlayers;
  const max = mode?.maxPlayers;
  if (!Number.isFinite(min) || !Number.isFinite(max)) {
    return null;
  }
  if (min === max) {
    return min === 1 ? "1 player" : `${min} players`;
  }
  return `${min}-${max} players`;
}

/**
 * Join UI options derived from expanded seat queue paths.
 * @returns {{ kind: 'fifo', paths: [] } | { kind: 'composition', paths: string[] }}
 */
export function joinGroupOptionsForMode(mode) {
  const queuePaths = (mode?.queuePaths ?? []).filter(
    (entry) => (entry.queuePath?.trim() ?? "") !== "",
  );
  if (queuePaths.length > 0) {
    return {
      kind: "composition",
      paths: queuePaths.map((entry) => ({
        queuePath: entry.queuePath,
        displayName: entry.displayName || entry.queuePath,
      })),
    };
  }

  const paths = new Set();
  for (const seat of mode?.seats ?? []) {
    const path = seat.queuePath?.trim() ?? "";
    if (path) {
      paths.add(path);
    }
  }
  const sorted = [...paths].sort();
  if (sorted.length === 0) {
    return { kind: "fifo", paths: [] };
  }
  return {
    kind: "composition",
    paths: sorted.map((queuePath) => ({ queuePath, displayName: queuePath })),
  };
}

export function joinGroupOptionsForGame(game) {
  return joinGroupOptionsForMode(defaultModeForGame(game));
}

export async function fetchGames(playerId = "") {
  const data = await graphqlRequest(GAMES_QUERY, {
    playerId,
    hasPlayer: Boolean(playerId),
  });
  return data.games ?? [];
}

// Only the roster, and only for one mode. The full game query is far too much to pull
// down for a picker, and QueueOptions is a live call out to the game — so this is fetched
// when the sheet opens rather than riding on every table or catalog payload (JQ-232).
const MODE_QUEUE_OPTIONS_QUERY = `
  query ModeQueueOptions($gameId: ID!, $playerId: ID!) {
    game(id: $gameId) {
      id
      modes {
        id
        queueOptions(playerId: $playerId) {
          available
          unavailableReason
          groups {
            key
            choices {
              id
              label
              description
              locked
            }
          }
        }
      }
    }
  }
`;

/**
 * This player's live roster for one mode, or null when the mode declares no groups.
 * The game is the only source: there is no safe fallback when it cannot be reached, so
 * an unavailable roster comes back as `available: false` for the sheet to say so.
 */
export async function fetchModeQueueOptions(gameId, modeId, playerId) {
  if (!gameId || !modeId || !playerId) {
    return null;
  }
  const data = await graphqlRequest(MODE_QUEUE_OPTIONS_QUERY, {
    gameId,
    playerId,
  });
  const mode = (data.game?.modes ?? []).find((entry) => entry.id === modeId);
  return mode?.queueOptions ?? null;
}

export async function fetchGameBySlug(slug, playerId = "") {
  const trimmed = String(slug || "").trim();
  if (!trimmed) {
    return null;
  }
  const data = await graphqlRequest(GAME_BY_SLUG_QUERY, {
    slug: trimmed,
    playerId,
    hasPlayer: Boolean(playerId),
  });
  return data.gameBySlug ?? null;
}

/**
 * Text a catalog search matches against: the game name plus its axis labels.
 *
 * Ids as well as labels, so typing "co-op" finds a game whose chip reads "Co-op"
 * and typing "words" finds one labelled "Words & Trivia".
 */
function gameSearchHaystack(game) {
  const modes = Array.isArray(game?.modes) ? game.modes : [];
  const parts = [
    game?.name ?? "",
    game?.genre ?? "",
    gameGenreLabel(game) ?? "",
    game?.difficulty ?? "",
    gameDifficultyLabel(game) ?? "",
    ...modes.flatMap((mode) => [
      mode?.socialMode ?? "",
      modeSocialModeLabel(mode) ?? "",
    ]),
  ];
  return parts.join(" ").toLowerCase();
}

/**
 * Filter catalog games by a free-text query.
 * Every whitespace-separated term must appear somewhere in the game's name or tags.
 * An empty query returns the list unchanged.
 */
export function filterGamesBySearch(games, query) {
  const list = Array.isArray(games) ? games : [];
  const terms = String(query || "")
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean);
  if (terms.length === 0) {
    return list;
  }
  return list.filter((game) => {
    const haystack = gameSearchHaystack(game);
    return terms.every((term) => haystack.includes(term));
  });
}
