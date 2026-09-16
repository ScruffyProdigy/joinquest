const CHECK_USER_ONE = 'a0000000-0000-4000-8000-000000000001'
const CHECK_USER_TWO = 'a0000000-0000-4000-8000-000000000002'

export function buildExampleProvisionPayload({
  game,
  credentials,
  lobbyIssuer,
  lobbyReturnUrl,
  lobbyGraphqlUrl,
}) {
  const mode = game?.modes?.[0]
  if (!mode) {
    throw new Error('Game has no synced modes — connect API and sync manifest first.')
  }

  const seats = mode.seats ?? []
  const seatCount = Math.min(2, seats.length)
  if (seatCount === 0) {
    throw new Error('Game mode has no expanded seats.')
  }

  const externalMatchId = `example-match-${game.id.slice(0, 8)}`
  const assignmentSeats = seats.slice(0, seatCount).map((seat, index) => ({
    seatKey: seat.seatKey,
    lobbyUserId: index === 0 ? CHECK_USER_ONE : CHECK_USER_TWO,
    displayName: index === 0 ? 'Player One' : 'Player Two',
  }))

  return {
    lobbyId: lobbyIssuer,
    lobby: {
      returnUrl: lobbyReturnUrl,
      graphqlUrl: lobbyGraphqlUrl,
      serviceToken: credentials.serviceToken,
    },
    assignment: {
      externalMatchId,
      gameMode: mode.modeKey,
      seats: assignmentSeats,
    },
  }
}

export function lobbyUrlsFromGraphQL(graphqlUrl) {
  const origin = graphqlUrl.replace(/\/graphql\/?$/, '')
  let publicUrl = origin
  if (origin.includes('localhost:8080')) {
    publicUrl = 'http://localhost:5173'
  }
  publicUrl = publicUrl.replace(/\/$/, '')
  return {
    lobbyIssuer: origin.replace(/\/$/, ''),
    lobbyReturnUrl: `${publicUrl}/return`,
    lobbyGraphqlUrl: `${origin.replace(/\/$/, '')}/graphql`,
  }
}

export function buildProvisionCurl(apiBaseUrl, payload) {
  const url = `${String(apiBaseUrl || '').replace(/\/$/, '')}/api/v1/matches`
  const body = JSON.stringify(payload, null, 2)
  return `curl -sS -X POST '${url}' \\
  -H 'Content-Type: application/json' \\
  -H 'Authorization: Bearer ${payload.lobby.serviceToken}' \\
  -d '${body.replace(/'/g, "'\\''")}'`
}
