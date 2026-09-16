import { graphqlRequest } from './graphql'
import { defaultModeForGame } from './games'

const MY_GAME_FIELDS = `
  id
  slug
  name
  shortDescription
  longDescription
  howToPlay
  genre
  difficulty
  accentColor
  apiBaseUrl
  visibility
  contactEmail
  websiteUrl
  communityUrl
  manifestSyncedAt
  integrationChecks {
    checkId
    status
    message
    detail
    ranAt
  }
  modes {
    id
    modeKey
    displayName
    socialMode
    status
    seats {
      seatKey
      queuePath
    }
    queues {
      id
      status
    }
  }
`

const CATALOG_AXIS_TAXONOMY_QUERY = `
  query CatalogAxisTaxonomy {
    genreTaxonomy {
      id
      label
      description
    }
    difficultyTaxonomy {
      id
      label
      description
    }
  }
`

const UPDATE_MY_GAME_METADATA = `
  mutation UpdateMyGameMetadata($input: UpdateMyGameMetadataInput!) {
    updateMyGameMetadata(input: $input) {
      ${MY_GAME_FIELDS}
    }
  }
`

const REQUEST_PUBLIC_RELEASE = `
  mutation RequestPublicRelease($gameId: ID!) {
    requestPublicRelease(gameId: $gameId) {
      id
      visibility
    }
  }
`

const MY_GAMES_QUERY = `
  query MyGames {
    myGames {
      id
      slug
      name
      shortDescription
      visibility
    }
  }
`

const MY_GAME_QUERY = `
  query MyGame($id: ID!) {
    myGame(id: $id) {
      ${MY_GAME_FIELDS}
    }
  }
`

const MY_GAME_CREDENTIALS_QUERY = `
  query MyGameCredentials($id: ID!) {
    myGameCredentials(id: $id) {
      serviceToken
      webhookSecret
    }
  }
`

const REGISTER_MY_GAME = `
  mutation RegisterMyGame($input: RegisterMyGameInput!) {
    registerMyGame(input: $input) {
      connected
      connectError
      webhookSecret
      serviceToken
      game {
        id
        slug
        name
        visibility
      }
    }
  }
`

const RUN_MY_GAME_CHECKS = `
  mutation RunMyGameChecks($gameId: ID!) {
    runMyGameChecks(gameId: $gameId) {
      checkId
      status
      message
      detail
      ranAt
    }
  }
`

const SYNC_MY_GAME_MANIFEST = `
  mutation SyncMyGameManifest($gameId: ID!) {
    syncMyGameManifest(gameId: $gameId) {
      changed
      connectError
      game {
        ${MY_GAME_FIELDS}
      }
    }
  }
`

const CONNECT_MY_GAME = `
  mutation ConnectMyGame($input: ConnectMyGameInput!) {
    connectMyGame(input: $input) {
      connected
      changed
      connectError
      game {
        ${MY_GAME_FIELDS}
      }
    }
  }
`

const ROTATE_MY_GAME_WEBHOOK_SECRET = `
  mutation RotateMyGameWebhookSecret($gameId: ID!) {
    rotateMyGameWebhookSecret(gameId: $gameId) {
      serviceToken
      webhookSecret
    }
  }
`

const INTEGRATION_GUIDE_QUERY = `
  query DeveloperIntegrationGuide {
    developerIntegrationGuide
  }
`

const MY_DEVELOPER_API_KEYS_QUERY = `
  query MyDeveloperApiKeys {
    myDeveloperApiKeys {
      id
      name
      keyPrefix
      createdAt
      lastUsedAt
    }
  }
`

const CREATE_DEVELOPER_API_KEY = `
  mutation CreateDeveloperApiKey($name: String) {
    createDeveloperApiKey(name: $name) {
      secret
      apiKey {
        id
        name
        keyPrefix
        createdAt
        lastUsedAt
      }
    }
  }
`

const REVOKE_DEVELOPER_API_KEY = `
  mutation RevokeDeveloperApiKey($id: ID!) {
    revokeDeveloperApiKey(id: $id)
  }
`

export const REQUIRED_INTEGRATION_CHECKS = [
  'manifest.reach_api',
  'manifest.status',
  'manifest.launch_urls_on_provision',
  'manifest.game_modes',
  'manifest.sync_freshness',
  'provision.happy_path',
  'provision.idempotent_repush',
  'provision.auth',
  'provision.missing_auth',
  'provision.launch_urls',
  'provision.launch_url_no_jwt',
  'jwt.jwks',
  'jwt.claim_happy_path',
  'jwt.wrong_audience',
  'jwt.unknown_match',
  'jwt.wrong_issuer',
  'jwt.expired',
  'jwt.invalid_token',
  'jwt.wrong_seat',
]

export const DEVELOPER_LANDING_PATH = '/developers'

/** Parse ?path= from /developers landing (manual browser registration vs AI setup). */
export function parseDeveloperLandingPath(search = '') {
  const raw = String(search || '')
  const params = new URLSearchParams(raw.startsWith('?') ? raw.slice(1) : raw)
  const path = params.get('path')
  if (path === 'manual' || path === 'ai') {
    return path
  }
  return null
}

export function developerLandingHref(path = null) {
  if (path === 'manual' || path === 'ai') {
    return `${DEVELOPER_LANDING_PATH}?path=${path}`
  }
  return DEVELOPER_LANDING_PATH
}

/** Parse developer routes from pathname. */
export function parseDeveloperRoute(pathname) {
  const path = String(pathname || '')
  if (path === '/developers' || path === '/developers/') {
    return { kind: 'landing' }
  }
  const welcomeMatch = path.match(/^\/developers\/games\/([^/]+)\/welcome\/?$/)
  if (welcomeMatch?.[1]) {
    return { kind: 'welcome', gameId: decodeURIComponent(welcomeMatch[1]) }
  }
  const dashboardMatch = path.match(/^\/developers\/games\/([^/]+)\/?$/)
  if (dashboardMatch?.[1]) {
    return { kind: 'dashboard', gameId: decodeURIComponent(dashboardMatch[1]) }
  }
  return null
}

export function suggestSlugFromName(name) {
  return String(name || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64)
}

export async function fetchMyGames() {
  const data = await graphqlRequest(MY_GAMES_QUERY)
  return data.myGames ?? []
}

export async function fetchMyGame(gameId) {
  const data = await graphqlRequest(MY_GAME_QUERY, { id: gameId })
  return data.myGame ?? null
}

export async function fetchMyGameCredentials(gameId) {
  const data = await graphqlRequest(MY_GAME_CREDENTIALS_QUERY, { id: gameId })
  return data.myGameCredentials ?? null
}

export async function registerMyGame(input) {
  const data = await graphqlRequest(REGISTER_MY_GAME, { input })
  return data.registerMyGame
}

export async function runMyGameChecks(gameId) {
  const data = await graphqlRequest(RUN_MY_GAME_CHECKS, { gameId })
  return data.runMyGameChecks ?? []
}

export async function syncMyGameManifest(gameId) {
  const data = await graphqlRequest(SYNC_MY_GAME_MANIFEST, { gameId })
  return data.syncMyGameManifest
}

export async function connectMyGame(input) {
  const data = await graphqlRequest(CONNECT_MY_GAME, { input })
  return data.connectMyGame
}

export async function rotateMyGameWebhookSecret(gameId) {
  const data = await graphqlRequest(ROTATE_MY_GAME_WEBHOOK_SECRET, { gameId })
  return data.rotateMyGameWebhookSecret
}

export async function fetchDeveloperIntegrationGuide() {
  const data = await graphqlRequest(INTEGRATION_GUIDE_QUERY)
  return data.developerIntegrationGuide ?? ''
}

/**
 * The axis vocabularies a developer picks from. Social mode is absent on purpose:
 * it is declared per mode in the game's own manifest, not edited here.
 */
export async function fetchCatalogAxisTaxonomy() {
  const data = await graphqlRequest(CATALOG_AXIS_TAXONOMY_QUERY)
  return {
    genre: data.genreTaxonomy ?? [],
    difficulty: data.difficultyTaxonomy ?? [],
  }
}

export async function updateMyGameMetadata(input) {
  const data = await graphqlRequest(UPDATE_MY_GAME_METADATA, { input })
  return data.updateMyGameMetadata
}

export async function requestPublicRelease(gameId) {
  const data = await graphqlRequest(REQUEST_PUBLIC_RELEASE, { gameId })
  return data.requestPublicRelease
}

export function integrationNextSteps(game) {
  if (!game) {
    return []
  }

  const checksById = new Map((game.integrationChecks ?? []).map((c) => [c.checkId, c]))
  const hasChecks = (game.integrationChecks ?? []).length > 0
  const requiredPass = REQUIRED_INTEGRATION_CHECKS.every(
    (id) => checksById.get(id)?.status === 'PASS',
  )
  const hasFailures = (game.integrationChecks ?? []).some(
    (c) => c.status === 'FAIL' && REQUIRED_INTEGRATION_CHECKS.includes(c.checkId),
  )
  const connected = game.visibility !== 'DRAFT'
  const hasMetadata =
    Boolean(game.shortDescription?.trim()) &&
    Boolean(game.longDescription?.trim()) &&
    Boolean(game.genre?.trim())
  const canRelease = canRequestPublicRelease(game)

  const steps = [
    {
      id: 'connect',
      label: 'Connect your game API',
      done: connected,
      hint: connected
        ? 'Your API is reachable and modes are synced.'
        : 'Deploy a public HTTPS URL with /healthz and /api/v1/game-modes, then use Connect API (or connectMyGame). Localhost will not work.',
    },
    {
      id: 'checks',
      label: 'Run integration checks',
      done: hasChecks && !hasFailures && requiredPass,
      hint: !connected
        ? 'Connect your API first.'
        : !hasChecks
          ? 'Click Run all checks on this dashboard.'
          : hasFailures
            ? 'Fix the failing checks below, then run checks again.'
            : 'All required checks passed.',
    },
    {
      id: 'metadata',
      label: 'Complete catalog listing',
      done: hasMetadata,
      hint: hasMetadata
        ? 'Short description, long description, and genre are set.'
        : 'Fill in catalog copy so players know what your game is about.',
    },
    {
      id: 'test',
      label: 'Create a test table',
      done: false,
      hint: 'Invite friends to your room and play before going public.',
      optional: !connected,
    },
    {
      id: 'release',
      label: 'Request public release',
      done: game.visibility === 'PENDING_REVIEW' || game.visibility === 'PUBLIC',
      hint: canRelease
        ? 'Ready when you are — we do a quick review before catalog listing.'
        : 'Complete checks and catalog metadata first.',
    },
  ]

  let foundCurrent = false
  return steps.map((step) => {
    if (step.optional) {
      return { ...step, status: 'upcoming' }
    }
    if (step.done) {
      return { ...step, status: 'done' }
    }
    if (!foundCurrent) {
      foundCurrent = true
      return { ...step, status: 'current' }
    }
    return { ...step, status: 'upcoming' }
  })
}

export async function fetchMyDeveloperApiKeys() {
  const data = await graphqlRequest(MY_DEVELOPER_API_KEYS_QUERY)
  return data.myDeveloperApiKeys ?? []
}

export async function createDeveloperApiKey(name) {
  const data = await graphqlRequest(CREATE_DEVELOPER_API_KEY, { name: name ?? null })
  return data.createDeveloperApiKey
}

export async function revokeDeveloperApiKey(id) {
  const data = await graphqlRequest(REVOKE_DEVELOPER_API_KEY, { id })
  return data.revokeDeveloperApiKey
}

export function canRequestPublicRelease(game) {
  if (!game || game.visibility !== 'PRIVATE_TESTING') {
    return false
  }
  const hasShort = Boolean(game.shortDescription?.trim())
  const hasLong = Boolean(game.longDescription?.trim())
  const hasGenre = Boolean(game.genre?.trim())
  const requiredChecks = REQUIRED_INTEGRATION_CHECKS
  const checksById = new Map((game.integrationChecks ?? []).map((c) => [c.checkId, c.status]))
  const checksPass = requiredChecks.every((id) => checksById.get(id) === 'PASS')
  return hasShort && hasLong && hasGenre && checksPass
}

export function defaultModeForMyGame(game) {
  return defaultModeForGame(game)
}

export function developerDashboardPath(gameId) {
  return `/developers/games/${encodeURIComponent(gameId)}`
}

export function developerWelcomePath(gameId) {
  return `/developers/games/${encodeURIComponent(gameId)}/welcome`
}
