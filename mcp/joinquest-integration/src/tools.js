import { z } from 'zod'
import {
  MUTATIONS,
  QUERIES,
  graphqlRequest,
} from './graphql.js'
import { buildExampleProvisionPayload } from './example-provision.js'

function textResult(value) {
  const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2)
  return { content: [{ type: 'text', text }] }
}

function lobbyUrls(apiUrl) {
  const origin = apiUrl.replace(/\/graphql\/?$/, '')
  const issuer = (process.env.JOINQUEST_ISSUER_URL || origin).replace(/\/$/, '')
  let publicUrl = process.env.JOINQUEST_PUBLIC_URL || origin
  if (origin.includes('localhost:8080') && !process.env.JOINQUEST_PUBLIC_URL) {
    publicUrl = 'http://localhost:5173'
  }
  publicUrl = publicUrl.replace(/\/$/, '')
  return {
    lobbyIssuer: issuer,
    lobbyReturnUrl: `${publicUrl}/return`,
    lobbyGraphqlUrl: `${issuer}/graphql`,
  }
}

export function registerJoinQuestIntegrationTools(server, config) {
  const { apiUrl, authHeader, cookieValue } = config
  const auth = authHeader ? { authHeader } : { cookieValue }
  const gql = (query, variables) => graphqlRequest(apiUrl, auth, query, variables)

  server.registerTool(
    'joinquest_integration_get_agent_playbook',
    {
      description:
        'Returns the end-to-end agent workflow for integrating a game with JoinQuest (phases 1–8: discovery, MCP setup, API implementation, registration, checks, metadata, test, release). Start here for vibe-coding integrations.',
      inputSchema: z.object({}),
    },
    async () => {
      const data = await gql(QUERIES.agentPlaybook)
      return textResult(data.developerAgentPlaybook)
    },
  )

  server.registerTool(
    'joinquest_integration_get_integration_guide',
    {
      description: 'Returns the full JoinQuest developer integration guide (markdown).',
      inputSchema: z.object({}),
    },
    async () => {
      const data = await gql(QUERIES.integrationGuide)
      return textResult(data.developerIntegrationGuide)
    },
  )

  server.registerTool(
    'joinquest_integration_get_discovery_prompt',
    {
      description:
        'Returns the agent discovery interview script: start open-ended, then ask clarifying questions for catalog copy and seatTemplate guidance.',
      inputSchema: z.object({}),
    },
    async () => {
      const data = await gql(QUERIES.discoveryPrompt)
      return textResult(data.developerDiscoveryPrompt)
    },
  )

  server.registerTool(
    'joinquest_integration_get_catalog_tag_taxonomy',
    {
      description:
        'Returns the catalog axis vocabularies: genre and difficulty (set per game via update_game_metadata) and socialMode (declared per mode in the game-modes manifest). Replaces the retired flat tag list.',
      inputSchema: z.object({}),
    },
    async () => {
      const data = await gql(QUERIES.catalogAxisTaxonomy)
      return textResult({
        genre: data.genreTaxonomy,
        difficulty: data.difficultyTaxonomy,
        socialMode: data.socialModeTaxonomy,
      })
    },
  )

  server.registerTool(
    'joinquest_integration_list_my_games',
    {
      description: "List games owned by the signed-in developer, including visibility state.",
      inputSchema: z.object({}),
    },
    async () => {
      const data = await gql(QUERIES.myGames)
      return textResult(data.myGames ?? [])
    },
  )

  server.registerTool(
    'joinquest_integration_register_game',
    {
      description:
        'Register a new game on JoinQuest for the authenticated developer. JoinQuest probes the public HTTPS apiBaseUrl (healthz + game-modes). Confirm field values with the developer before calling. Returns game id, visibility, serviceToken, webhookSecret, and connectError if the API is unreachable.',
      inputSchema: z.object({
        name: z.string().describe('Display name'),
        slug: z.string().describe('Lowercase URL slug (unique on JoinQuest)'),
        shortDescription: z.string().describe('Initial catalog card blurb (~1–2 sentences)'),
        apiBaseUrl: z
          .string()
          .describe('Public HTTPS origin for the game API, e.g. https://mygame.example.com'),
        contactEmail: z.string().describe('Email for review notifications'),
        websiteUrl: z.string().optional().describe('Optional marketing or docs URL'),
        communityUrl: z.string().optional().describe('Optional Discord or community URL'),
      }),
    },
    async ({
      name,
      slug,
      shortDescription,
      apiBaseUrl,
      contactEmail,
      websiteUrl,
      communityUrl,
    }) => {
      const input = {
        name,
        slug,
        shortDescription,
        apiBaseUrl,
        contactEmail,
      }
      if (websiteUrl !== undefined) {
        input.websiteUrl = websiteUrl
      }
      if (communityUrl !== undefined) {
        input.communityUrl = communityUrl
      }

      const data = await gql(MUTATIONS.registerMyGame, { input })
      return textResult(data.registerMyGame)
    },
  )

  server.registerTool(
    'joinquest_integration_get_game_checks',
    {
      description: 'Fetch latest integration checklist results and catalog metadata for one owned game.',
      inputSchema: z.object({
        gameId: z.string().describe('JoinQuest game UUID'),
      }),
    },
    async ({ gameId }) => {
      const data = await gql(QUERIES.myGame, { id: gameId })
      if (!data.myGame) {
        throw new Error('Game not found or not owned by this session.')
      }
      return textResult(data.myGame)
    },
  )

  server.registerTool(
    'joinquest_integration_run_game_checks',
    {
      description: 'Run manifest, provision, and JWT integration checks for an owned game.',
      inputSchema: z.object({
        gameId: z.string().describe('JoinQuest game UUID'),
      }),
    },
    async ({ gameId }) => {
      const data = await gql(MUTATIONS.runMyGameChecks, { gameId })
      return textResult(data.runMyGameChecks ?? [])
    },
  )

  server.registerTool(
    'joinquest_integration_sync_game_manifest',
    {
      description:
        'Re-fetch /api/v1/game-modes from the game apiBaseUrl and refresh JoinQuest’s cached modes/seats. Call this after fixing seatTemplate (or other manifest fields) before re-running checks. Promotes draft → private_testing on success.',
      inputSchema: z.object({
        gameId: z.string().describe('JoinQuest game UUID'),
      }),
    },
    async ({ gameId }) => {
      const data = await gql(MUTATIONS.syncMyGameManifest, { gameId })
      return textResult(data.syncMyGameManifest)
    },
  )

  server.registerTool(
    'joinquest_integration_connect_game',
    {
      description:
        'Connect or reconnect the game API. Optionally set a new public HTTPS apiBaseUrl. Use for draft games that failed first connect, or when the API host changes. Confirm the URL with the developer before changing it. Cannot change apiBaseUrl while pending_review or public.',
      inputSchema: z.object({
        gameId: z.string().describe('JoinQuest game UUID'),
        apiBaseUrl: z
          .string()
          .optional()
          .describe('Optional new public HTTPS origin; omit to retry the current URL'),
      }),
    },
    async ({ gameId, apiBaseUrl }) => {
      const input = { gameId }
      if (apiBaseUrl !== undefined) {
        input.apiBaseUrl = apiBaseUrl
      }
      const data = await gql(MUTATIONS.connectMyGame, { input })
      return textResult(data.connectMyGame)
    },
  )

  server.registerTool(
    'joinquest_integration_rotate_webhook_secret',
    {
      description:
        'Rotate the webhook secret for an owned game. The previous secret stops working immediately. Confirm with the developer before calling (sensitive).',
      inputSchema: z.object({
        gameId: z.string().describe('JoinQuest game UUID'),
      }),
    },
    async ({ gameId }) => {
      const data = await gql(MUTATIONS.rotateMyGameWebhookSecret, { gameId })
      return textResult(data.rotateMyGameWebhookSecret)
    },
  )

  server.registerTool(
    'joinquest_integration_update_game_metadata',
    {
      description:
        'Save catalog listing copy, display name, contact/links, genre and difficulty after developer approval. Use IDs from get_catalog_tag_taxonomy. Social mode is per-mode and belongs in the game-modes manifest, not here. Empty websiteUrl/communityUrl/accentColor/genre/difficulty clears those fields.',
      inputSchema: z.object({
        gameId: z.string(),
        name: z.string().optional(),
        shortDescription: z.string().optional(),
        longDescription: z.string().optional(),
        howToPlay: z.string().optional(),
        genre: z.string().optional(),
        difficulty: z.string().optional(),
        contactEmail: z.string().optional(),
        websiteUrl: z.string().optional(),
        communityUrl: z.string().optional(),
        accentColor: z.string().optional(),
      }),
    },
    async ({
      gameId,
      name,
      shortDescription,
      longDescription,
      howToPlay,
      genre,
      difficulty,
      contactEmail,
      websiteUrl,
      communityUrl,
      accentColor,
    }) => {
      const input = { gameId }
      if (name !== undefined) input.name = name
      if (shortDescription !== undefined) input.shortDescription = shortDescription
      if (longDescription !== undefined) input.longDescription = longDescription
      if (howToPlay !== undefined) input.howToPlay = howToPlay
      if (genre !== undefined) input.genre = genre
      if (difficulty !== undefined) input.difficulty = difficulty
      if (contactEmail !== undefined) input.contactEmail = contactEmail
      if (websiteUrl !== undefined) input.websiteUrl = websiteUrl
      if (communityUrl !== undefined) input.communityUrl = communityUrl
      if (accentColor !== undefined) input.accentColor = accentColor

      const data = await gql(MUTATIONS.updateMyGameMetadata, { input })
      return textResult(data.updateMyGameMetadata)
    },
  )

  server.registerTool(
    'joinquest_integration_get_game_credentials',
    {
      description: 'Return serviceToken and webhookSecret for an owned game (sensitive).',
      inputSchema: z.object({
        gameId: z.string(),
      }),
    },
    async ({ gameId }) => {
      const data = await gql(QUERIES.myGameCredentials, { id: gameId })
      if (!data.myGameCredentials) {
        throw new Error('Credentials not found or game not owned by this session.')
      }
      return textResult(data.myGameCredentials)
    },
  )

  server.registerTool(
    'joinquest_integration_get_example_provision_payload',
    {
      description: 'Build a sample POST /api/v1/matches body for local testing, using synced modes and credentials.',
      inputSchema: z.object({
        gameId: z.string(),
      }),
    },
    async ({ gameId }) => {
      const [gameData, credData] = await Promise.all([
        gql(QUERIES.myGame, { id: gameId }),
        gql(QUERIES.myGameCredentials, { id: gameId }),
      ])
      if (!gameData.myGame || !credData.myGameCredentials) {
        throw new Error('Game or credentials not found.')
      }
      const urls = lobbyUrls(apiUrl)
      const payload = buildExampleProvisionPayload({
        game: gameData.myGame,
        credentials: credData.myGameCredentials,
        ...urls,
      })
      return textResult(payload)
    },
  )

  server.registerTool(
    'joinquest_integration_request_public_release',
    {
      description: 'Submit an owned game for public catalog review when checklist and metadata gates pass.',
      inputSchema: z.object({
        gameId: z.string(),
      }),
    },
    async ({ gameId }) => {
      const data = await gql(MUTATIONS.requestPublicRelease, { gameId })
      return textResult(data.requestPublicRelease)
    },
  )
}
