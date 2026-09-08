#!/usr/bin/env node
/**
 * JQ-203 guard: catalog identity has one writer, and every handoff URL points at
 * the game its card advertises.
 *
 * Production shipped two "Rock Paper Scissors Lizard Robot" cards because the
 * k8s patch job reassigned slugs and names after the migrations had already
 * written identity onto the seed ids. The job runs after migrations on every
 * deploy, so whatever it sets wins silently. Two rules keep that from recurring:
 *
 *   1. k8s/jobs/patch-game-handoff-urls.yaml may not write identity columns.
 *      Identity belongs to the migration named below.
 *   2. The api_base_url the job assigns to a seed id must belong to the game
 *      whose slug that migration pins to the same id.
 *
 * Runs without a database: both sources are files in the tree.
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')

const JOB_PATH = 'k8s/jobs/patch-game-handoff-urls.yaml'
const MIGRATION_PATH = 'backend/migrations/000050_catalog_identity_by_slug.up.sql'
const DEPLOY_VERIFY_PATH = 'scripts/verify-game-deployments.sh'

/** Columns that describe *which game a row is*. Only the migration may set them. */
const IDENTITY_COLUMNS = [
  'slug',
  'name',
  'description',
  'short_description',
  'how_to_play',
  'tutorial_url',
  'icon_url',
  'hero_url',
  'catalog_hero_url',
  'title_url',
  'title_anchor',
  'title_width_pct',
  'tags',
]

const errors = []

function read(relPath) {
  const abs = path.join(root, relPath)
  if (!fs.existsSync(abs)) {
    console.error(`Missing ${relPath}`)
    process.exit(1)
  }
  return fs.readFileSync(abs, 'utf8')
}

/** Strip `--` comments so commented-out SQL is not mistaken for a statement. */
function stripSqlComments(sql) {
  return sql.replace(/--[^\n]*/g, '')
}

/** Every `UPDATE games SET <assignments> WHERE id = '<uuid>'` in a SQL blob. */
function parseGameUpdatesByID(sql) {
  const updates = []
  const re = /UPDATE\s+games\s+SET\s+([\s\S]*?)WHERE\s+id\s*=\s*'([0-9a-f-]+)'/gi
  for (const m of sql.matchAll(re)) {
    updates.push({ assignments: m[1], id: m[2].toLowerCase() })
  }
  return updates
}

/** Column names on the left of each `col = value` assignment. */
function assignedColumns(assignments) {
  return [...assignments.matchAll(/(?:^|,)\s*([a-z_]+)\s*=/gi)].map((m) => m[1].toLowerCase())
}

/**
 * Which host serves which game, read from the deploy verifier rather than
 * restated here. That script already declares the pairing and checks it against
 * the live server (`verify_host <host> <slug>`); this one checks that the
 * catalog row for the same slug points at that host. Two sides of AC-4 from a
 * single declaration.
 *
 * The pairing has to be declared somewhere: "rpsls-duel.win" shares no token
 * with "rock-paper-scissors-lizard-robot", so a rule loose enough to derive it
 * from the slug would have accepted the bug too.
 */
function parseExpectedHandoffHosts(source) {
  const hosts = new Map()
  for (const m of source.matchAll(/^\s*verify_host\s+(\S+)\s+(\S+)/gm)) {
    hosts.set(m[2], m[1])
  }
  return hosts
}

const EXPECTED_HANDOFF_HOSTS = parseExpectedHandoffHosts(read(DEPLOY_VERIFY_PATH))

if (EXPECTED_HANDOFF_HOSTS.size === 0) {
  errors.push(`${DEPLOY_VERIFY_PATH}: no "verify_host <host> <slug>" lines found — the host↔slug pairing is gone.`)
}

// --- Rule 1: the job writes no identity ------------------------------------

const jobSource = stripSqlComments(read(JOB_PATH))

for (const { assignments, id } of parseGameUpdatesByID(jobSource)) {
  for (const column of assignedColumns(assignments)) {
    if (IDENTITY_COLUMNS.includes(column)) {
      errors.push(
        `${JOB_PATH}: sets identity column "${column}" on ${id}. ` +
          `Identity belongs to ${MIGRATION_PATH}; the job runs after migrations and would override it.`,
      )
    }
  }
}

// --- Rule 2: each id's handoff host matches the slug pinned to that id ------

const migrationSource = stripSqlComments(read(MIGRATION_PATH))

const slugByID = new Map()
for (const { assignments, id } of parseGameUpdatesByID(migrationSource)) {
  const slug = assignments.match(/slug\s*=\s*'([^']+)'/i)
  if (slug) slugByID.set(id, slug[1])
}

if (slugByID.size === 0) {
  errors.push(`${MIGRATION_PATH}: no "slug = '...' WHERE id = '...'" pins found — the id→slug source of truth is gone.`)
}

const slugsSeen = new Map()
for (const [id, slug] of slugByID) {
  if (slugsSeen.has(slug)) {
    errors.push(`${MIGRATION_PATH}: slug "${slug}" is pinned to both ${slugsSeen.get(slug)} and ${id}.`)
  }
  slugsSeen.set(slug, id)
}

for (const { assignments, id } of parseGameUpdatesByID(jobSource)) {
  const apiBaseURL = assignments.match(/api_base_url\s*=\s*'([^']+)'/i)
  if (!apiBaseURL) continue

  const slug = slugByID.get(id)
  if (!slug) {
    errors.push(`${JOB_PATH}: sets api_base_url on ${id}, but ${MIGRATION_PATH} pins no slug to that id.`)
    continue
  }

  const expected = EXPECTED_HANDOFF_HOSTS.get(slug)
  if (!expected) {
    errors.push(
      `Slug "${slug}" (id ${id}) has no verify_host line in ${DEPLOY_VERIFY_PATH}. ` +
        `Add one so its handoff host is both declared and checked against the live server.`,
    )
    continue
  }

  let actual
  try {
    actual = new URL(apiBaseURL[1]).host
  } catch {
    errors.push(`${JOB_PATH}: api_base_url "${apiBaseURL[1]}" on ${id} is not a valid URL.`)
    continue
  }

  if (actual !== expected) {
    errors.push(
      `${JOB_PATH}: ${id} advertises "${slug}" but hands off to ${actual} (expected ${expected}). ` +
        `Players would be sent to a different game than the card shows.`,
    )
  }
}

// --- Report ----------------------------------------------------------------

if (errors.length) {
  console.error('Catalog identity check failed:')
  for (const error of errors) console.error(`  • ${error}`)
  process.exit(1)
}

const pins = [...slugByID].map(([, slug]) => `${slug} → ${EXPECTED_HANDOFF_HOSTS.get(slug) ?? '(no handoff)'}`)
console.log(`OK: catalog identity has one writer; ${pins.length} seeded card(s) hand off correctly (${pins.join(', ')})`)
