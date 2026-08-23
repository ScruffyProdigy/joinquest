export async function graphqlRequest(apiUrl, auth, query, variables = {}) {
  const headers = {
    'Content-Type': 'application/json',
  }
  if (auth.authHeader) {
    headers.Authorization = auth.authHeader
  } else if (auth.cookieValue) {
    headers.Cookie = auth.cookieValue
  }

  const response = await fetch(apiUrl, {
    method: 'POST',
    headers,
    body: JSON.stringify({ query, variables }),
  })

  let payload
  try {
    payload = await response.json()
  } catch {
    throw new Error(`JoinQuest API returned non-JSON (${response.status})`)
  }

  if (!response.ok) {
    const detail = payload.errors?.[0]?.message || payload.error || response.statusText
    throw new Error(detail || `API request failed (${response.status})`)
  }

  if (payload.errors?.length) {
    throw new Error(payload.errors[0]?.message || 'GraphQL request failed')
  }

  return payload.data
}

export const QUERIES = {
  agentPlaybook: `
    query DeveloperAgentPlaybook {
      developerAgentPlaybook
    }
  `,
  integrationGuide: `
    query DeveloperIntegrationGuide {
      developerIntegrationGuide
    }
  `,
  discoveryPrompt: `
    query DeveloperDiscoveryPrompt {
      developerDiscoveryPrompt
    }
  `,
  catalogTagTaxonomy: `
    query CatalogTagTaxonomy {
      catalogTagTaxonomy {
        id
        label
        description
      }
    }
  `,
  myGames: `
    query MyGames {
      myGames {
        id
        slug
        name
        shortDescription
        visibility
        apiBaseUrl
      }
    }
  `,
  myGame: `
    query MyGame($id: ID!) {
      myGame(id: $id) {
        id
        slug
        name
        shortDescription
        longDescription
        howToPlay
        tags
        visibility
        apiBaseUrl
        integrationChecks {
          checkId
          status
          message
          detail
          ranAt
        }
        modes {
          modeKey
          displayName
          minPlayers
          maxPlayers
          seats {
            seatKey
            queuePath
          }
        }
      }
    }
  `,
  myGameCredentials: `
    query MyGameCredentials($id: ID!) {
      myGameCredentials(id: $id) {
        serviceToken
        webhookSecret
      }
    }
  `,
}

export const MUTATIONS = {
  runMyGameChecks: `
    mutation RunMyGameChecks($gameId: ID!) {
      runMyGameChecks(gameId: $gameId) {
        checkId
        status
        message
        detail
        ranAt
      }
    }
  `,
  updateMyGameMetadata: `
    mutation UpdateMyGameMetadata($input: UpdateMyGameMetadataInput!) {
      updateMyGameMetadata(input: $input) {
        id
        name
        shortDescription
        longDescription
        howToPlay
        tags
        contactEmail
        websiteUrl
        communityUrl
        accentColor
      }
    }
  `,
  syncMyGameManifest: `
    mutation SyncMyGameManifest($gameId: ID!) {
      syncMyGameManifest(gameId: $gameId) {
        changed
        connectError
        game {
          id
          visibility
          apiBaseUrl
          manifestSyncedAt
          modes {
            modeKey
            displayName
            minPlayers
            maxPlayers
            seats {
              seatKey
              queuePath
            }
          }
        }
      }
    }
  `,
  connectMyGame: `
    mutation ConnectMyGame($input: ConnectMyGameInput!) {
      connectMyGame(input: $input) {
        connected
        changed
        connectError
        game {
          id
          visibility
          apiBaseUrl
          manifestSyncedAt
          modes {
            modeKey
            displayName
            minPlayers
            maxPlayers
          }
        }
      }
    }
  `,
  rotateMyGameWebhookSecret: `
    mutation RotateMyGameWebhookSecret($gameId: ID!) {
      rotateMyGameWebhookSecret(gameId: $gameId) {
        serviceToken
        webhookSecret
      }
    }
  `,
  requestPublicRelease: `
    mutation RequestPublicRelease($gameId: ID!) {
      requestPublicRelease(gameId: $gameId) {
        id
        visibility
      }
    }
  `,
  registerMyGame: `
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
          apiBaseUrl
        }
      }
    }
  `,
}
