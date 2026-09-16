/**
 * Display strings for the developer dashboard: how a visibility enum, an
 * integration check id, or a check status is worded in the UI, and the fix hint
 * shown under a failing check. No API calls, no state — just copy.
 */

export const CHECK_FIX_HINTS = {
  'manifest.reach_api':
    'JoinQuest must reach your public HTTPS API. Check the URL is live and not localhost.',
  'manifest.status':
    'GET /api/v1/status should return game info and launchUrlsOnProvision: true.',
  'manifest.launch_urls_on_provision':
    'GET /api/v1/status must include launchUrlsOnProvision: true.',
  'manifest.game_modes':
    'GET /api/v1/game-modes needs valid seatTemplate JSON for each mode.',
  'manifest.sync_freshness':
    'Call syncMyGameManifest (dashboard: Resync game modes) after deploying API changes, then re-run checks.',
  'provision.happy_path':
    'POST /api/v1/matches should return launch URLs for each seated player.',
  'provision.idempotent_repush':
    'Re-posting the same externalMatchId should succeed (idempotent provision).',
  'provision.auth':
    'Accept the service token on Authorization: Bearer … when provisioning matches.',
  'provision.missing_auth':
    'Reject provision requests with no Authorization header.',
  'provision.banlist':
    'Return HTTP 403 with bannedLobbyUserIds when a player is banned.',
  'provision.launch_urls':
    'Every seated player needs a launchUrls entry (or a launchUrlTemplate).',
  'provision.launch_url_no_jwt':
    'Launch URL bases must not include a JWT — JoinQuest adds token= later.',
  'jwt.jwks': 'Publish JWKS at {lobby}/.well-known/jwks.json.',
  'jwt.claim_happy_path': 'POST /api/v1/matches/{id}/claim must accept Lobby seat JWTs.',
  'jwt.wrong_audience': 'Reject JWTs whose aud does not match your API base URL.',
  'jwt.unknown_match': 'Return 404 when the claim URL match id is unknown or mismatched.',
  'jwt.wrong_issuer': 'Reject JWTs whose iss does not match the match lobbyId.',
  'jwt.expired': 'Reject expired seat tokens with 401/403.',
  'jwt.invalid_token': 'Reject malformed tokens with 401/403.',
  'jwt.wrong_seat': 'Reject tokens that claim another player\'s reserved seat.',
  'jwt.reclaim_same_player':
    'Compare the token sub against the player already in the seat: same sub is a reconnect (200), a different sub is the conflict (409). A flat 409 locks players out of their own match.',
  'jwt.reclaim_seat_theft':
    'A different player claiming an occupied seat must be refused with 409 (401/403 also fine).',
  'jwt.rotation_overlap':
    "On an unrecognized kid, refetch JWKS (rate-limited) before rejecting — don't verify against a single cached key.",
}

export function checkFixHint(checkId) {
  return CHECK_FIX_HINTS[checkId] ?? 'See the integration guide below for details.'
}

export function visibilityLabel(visibility) {
  switch (visibility) {
    case 'PRIVATE_TESTING':
      return 'Private testing'
    case 'PENDING_REVIEW':
      return 'Pending review'
    case 'PUBLIC':
      return 'Public'
    default:
      return 'Draft'
  }
}

export function checkSectionTitle(checkId) {
  const section = String(checkId || '').split('.')[0]
  switch (section) {
    case 'manifest':
      return 'Manifest'
    case 'provision':
      return 'Provisioning'
    case 'jwt':
      return 'JWT verification'
    default:
      return 'Checks'
  }
}

export function checkStatusLabel(status) {
  switch (status) {
    case 'PASS':
      return 'Pass'
    case 'FAIL':
      return 'Fail'
    default:
      return 'Skipped'
  }
}
