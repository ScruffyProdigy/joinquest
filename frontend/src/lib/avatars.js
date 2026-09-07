import { graphqlRequest } from './graphql'
import { displayNameOrFallback } from './viewer'

export const USER_AVATAR_FIELDS = `
  id
  displayName
  avatarUrl
  avatarKey
`

export const STARTER_AVATARS_QUERY = `
  query StarterAvatars {
    starterAvatars {
      key
      name
      slot
      imageUrl
    }
  }
`

const UPDATE_PLAYER_PROFILE = `
  mutation UpdatePlayerProfile($displayName: String!, $avatarKey: ID) {
    updatePlayerProfile(displayName: $displayName, avatarKey: $avatarKey) {
      id
      email
      displayName
      avatarUrl
      avatarKey
      avatarSource
      createdAt
    }
  }
`

/** Static fallback when the catalog query is unavailable (tests, offline). */
export const STARTER_AVATAR_FALLBACK = [
  { key: 'compass', name: 'Compass', slot: 'Compass', imageUrl: '/avatars/compass.png' },
  { key: 'coin', name: 'Coin', slot: 'Coin', imageUrl: '/avatars/coin.png' },
  { key: 'storm', name: 'Storm', slot: 'Storm', imageUrl: '/avatars/storm.png' },
  { key: 'campfire', name: 'Campfire', slot: 'Campfire', imageUrl: '/avatars/campfire.png' },
  { key: 'beacon', name: 'Beacon', slot: 'Beacon', imageUrl: '/avatars/beacon.png' },
  { key: 'angel', name: 'Angel', slot: 'Angel', imageUrl: '/avatars/angel.png' },
  { key: 'bell', name: 'Bell', slot: 'Bell', imageUrl: '/avatars/bell.png' },
  { key: 'constellation', name: 'Constellation', slot: 'Constellation', imageUrl: '/avatars/constellation.png' },
  { key: 'crown', name: 'Crown', slot: 'Crown', imageUrl: '/avatars/crown.png' },
  { key: 'footsteps', name: 'Footsteps', slot: 'Footsteps', imageUrl: '/avatars/footsteps.png' },
  { key: 'forge', name: 'Forge', slot: 'Forge', imageUrl: '/avatars/forge.png' },
  { key: 'goblet', name: 'Goblet', slot: 'Goblet', imageUrl: '/avatars/goblet.png' },
  { key: 'jester', name: 'Jester', slot: 'Jester', imageUrl: '/avatars/jester.png' },
  { key: 'key', name: 'Key', slot: 'Key', imageUrl: '/avatars/key.png' },
  { key: 'kite', name: 'Kite', slot: 'Kite', imageUrl: '/avatars/kite.png' },
  { key: 'moon', name: 'Moon', slot: 'Moon', imageUrl: '/avatars/moon.png' },
  { key: 'ring', name: 'Ring', slot: 'Ring', imageUrl: '/avatars/ring.png' },
  { key: 'rope', name: 'Rope', slot: 'Rope', imageUrl: '/avatars/rope.png' },
]

export function resolveUserAvatarUrl(user) {
  const direct = user?.avatarUrl?.trim()
  if (direct) {
    return direct
  }
  const key = user?.avatarKey?.trim()
  if (!key) {
    return null
  }
  const known = STARTER_AVATAR_FALLBACK.find((item) => item.key === key)
  return known?.imageUrl ?? `/avatars/${key}.png`
}

/** What the editor should prefill — empty for a player who has no name yet. */
export function defaultDisplayNameInput(user) {
  return user?.displayName?.trim() || ''
}

export async function fetchStarterAvatars() {
  try {
    const data = await graphqlRequest(STARTER_AVATARS_QUERY)
    return data.starterAvatars?.length ? data.starterAvatars : STARTER_AVATAR_FALLBACK
  } catch {
    return STARTER_AVATAR_FALLBACK
  }
}

export async function updatePlayerProfile(displayName, avatarKey) {
  const variables = { displayName }
  if (avatarKey?.trim()) {
    variables.avatarKey = avatarKey.trim()
  }
  const data = await graphqlRequest(UPDATE_PLAYER_PROFILE, variables)
  return data.updatePlayerProfile
}

export function avatarInitial(user) {
  return displayNameOrFallback(user).charAt(0).toUpperCase()
}
