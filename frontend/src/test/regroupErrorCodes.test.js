import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { buildSchema } from 'graphql'
import { describe, expect, it } from 'vitest'

import { REGROUP_ERROR } from '../lib/matchResult'

/**
 * The regroup failures reach the client as a `code` extension on the GraphQL error, and
 * nothing in a GraphQL document mentions them — so the document validation in
 * graphqlDocuments.test.js cannot see this contract at all.
 *
 * This test closes that gap: the codes `classifyRegroupError` switches on must all exist
 * in the backend's `RegroupErrorCode` enum. Renaming or dropping one on either side fails
 * here, which is what keeps the frontend's `NO_MODE` fallback from silently degrading to
 * the generic "try again" (JQ-176).
 */

const testDir = path.dirname(fileURLToPath(import.meta.url))
const schemaDir = path.resolve(testDir, '../../../backend/graph/schema')

function loadSchema() {
  const sdl = readdirSync(schemaDir)
    .filter((name) => name.endsWith('.graphqls'))
    .sort()
    .map((name) => readFileSync(path.join(schemaDir, name), 'utf8'))
    .join('\n')
  return buildSchema(sdl)
}

// UNKNOWN is the client's own "the backend sent no code", not an enum member.
const clientCodes = Object.entries(REGROUP_ERROR)
  .filter(([key]) => key !== 'UNKNOWN')
  .map(([, value]) => value)
  .sort()

describe('regroup error codes', () => {
  const enumType = loadSchema().getType('RegroupErrorCode')

  it('is declared as an enum in the backend schema', () => {
    expect(enumType).toBeDefined()
  })

  it('declares every code the client switches on', () => {
    const schemaCodes = enumType.getValues().map((value) => value.name).sort()
    expect(schemaCodes).toEqual(clientCodes)
  })
})
