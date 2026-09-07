import { execSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..')

/** Seeded catalog game 001 (Word Hunt in production; E2E uses localhost handoff URLs). */
export const DEMO_PRIMARY_GAME_ID = 'a1000000-0000-4000-8000-000000000001'
export const DEMO_DEFAULT_QUEUE_ID = 'a3000000-0000-4000-8000-000000000001'

const DEFAULT_API_BASE_URL = 'http://localhost:3001'

function psqlAvailable() {
  try {
    execSync('command -v psql', { stdio: 'ignore' })
    return true
  } catch {
    return false
  }
}

/** Run a single SQL statement against the E2E database. */
export function runSql(sql) {
  if (process.env.DATABASE_URL && psqlAvailable()) {
    return execSync(`psql "${process.env.DATABASE_URL}" -v ON_ERROR_STOP=1 -c "${sql}"`, {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    })
  }

  return execSync(`docker compose exec -T postgres psql -U app -d joinquest -v ON_ERROR_STOP=1 -c "${sql}"`, {
    cwd: repoRoot,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  })
}

export function setPrimaryGameAPIBaseUrl(apiBaseUrl) {
  runSql(`UPDATE games SET api_base_url='${apiBaseUrl}' WHERE id='${DEMO_PRIMARY_GAME_ID}'`)
}

export function restorePrimaryGameHandoffUrls() {
  setPrimaryGameAPIBaseUrl(DEFAULT_API_BASE_URL)
}

/** Reset demo queue rows and active sessions between E2E runs. */
export function clearDemoMatchmakingState() {
  runSql(`
    UPDATE game_queues
    SET status = 'cancelled'
    WHERE mode_queue_id = '${DEMO_DEFAULT_QUEUE_ID}'
      AND status IN ('waiting', 'matched');
  `)
  runSql(`
    UPDATE game_sessions
    SET status = 'completed', ended_at = NOW()
    WHERE mode_queue_id = '${DEMO_DEFAULT_QUEUE_ID}'
      AND status = 'active';
  `)
}
