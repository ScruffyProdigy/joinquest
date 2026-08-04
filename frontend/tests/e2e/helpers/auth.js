import { execSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect } from '@playwright/test'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..')
const E2E_BACKEND_LOG = path.join(repoRoot, 'tmp/e2e-backend.log')
const GRAPHQL_URL = process.env.E2E_GRAPHQL_URL || 'http://127.0.0.1:8080/graphql'
const SESSION_COOKIE_NAME = 'lobby_session'

const REQUEST_SIGN_IN = `
  mutation RequestSignIn($email: String!) {
    requestSignIn(email: $email)
  }
`

const COMPLETE_SIGN_IN_WITH_LINK = `
  mutation CompleteSignInWithLink($token: ID!) {
    completeSignInWithLink(token: $token) {
      id
      email
    }
  }
`

const COMPLETE_SIGN_IN_WITH_CODE = `
  mutation CompleteSignInWithCode($email: String!, $code: String!) {
    completeSignInWithCode(email: $email, code: $code) {
      id
      email
    }
  }
`

function psqlAvailable() {
  try {
    execSync('command -v psql', { stdio: 'ignore' })
    return true
  } catch {
    return false
  }
}

function runPsql(sql) {
  if (process.env.DATABASE_URL && psqlAvailable()) {
    return execSync(`psql "${process.env.DATABASE_URL}" -tAc "${sql}"`, {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    }).trim()
  }

  return execSync(`docker compose exec -T postgres psql -U app -d playhub -tAc "${sql}"`, {
    cwd: repoRoot,
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  }).trim()
}

function readBackendLog() {
  if (!fs.existsSync(E2E_BACKEND_LOG)) {
    throw new Error(`E2E backend log not found at ${E2E_BACKEND_LOG}`)
  }
  return fs.readFileSync(E2E_BACKEND_LOG, 'utf8')
}

function tokenFromMagicLinkUrl(link) {
  const url = new URL(link)
  const token = url.searchParams.get('token')
  if (!token) {
    throw new Error(`No token query param in magic link URL: ${link}`)
  }
  return token
}

function latestMagicLinkTokenFromLog(email) {
  const normalizedEmail = email.trim().toLowerCase()
  const escapedEmail = normalizedEmail.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const patterns = [
    new RegExp(`email: sign-in for ${escapedEmail} code=\\d{6} link=(\\S+)`, 'g'),
    new RegExp(`email: sign-in for ${escapedEmail} -> (\\S+)`, 'g'),
  ]
  const content = readBackendLog()

  let lastLink = ''
  for (const pattern of patterns) {
    let match
    while ((match = pattern.exec(content)) !== null) {
      lastLink = match[1]
    }
  }

  if (!lastLink) {
    throw new Error(`No magic link found in E2E log for ${email}`)
  }

  return tokenFromMagicLinkUrl(lastLink)
}

function latestLoginCodeFromLog(email) {
  const normalizedEmail = email.trim().toLowerCase()
  const escapedEmail = normalizedEmail.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const pattern = new RegExp(`email: sign-in for ${escapedEmail} code=(\\d{6})`, 'g')
  const content = readBackendLog()

  let match
  let lastCode = ''
  while ((match = pattern.exec(content)) !== null) {
    lastCode = match[1]
  }

  if (!lastCode) {
    throw new Error(`No sign-in code found in E2E log for ${email}`)
  }

  return lastCode
}

async function pollForValue(readValue, label, { timeoutMs = 10000 } = {}) {
  const deadline = Date.now() + timeoutMs

  while (Date.now() < deadline) {
    try {
      return readValue()
    } catch {
      await new Promise((resolve) => {
        setTimeout(resolve, 200)
      })
    }
  }

  throw new Error(`Timed out waiting for ${label}`)
}

function sessionCookieFromHeaders(headers) {
  const rawCookies =
    typeof headers.getSetCookie === 'function'
      ? headers.getSetCookie()
      : [headers.get('set-cookie')].filter(Boolean)

  for (const header of rawCookies) {
    if (!header) continue
    const [pair] = header.split(';')
    const eq = pair.indexOf('=')
    if (eq === -1) continue
    const name = pair.slice(0, eq).trim()
    if (name === SESSION_COOKIE_NAME) {
      return pair.slice(eq + 1)
    }
  }

  return null
}

async function nodeGraphqlRequest(query, variables = {}, sessionCookie = '') {
  const headers = { 'Content-Type': 'application/json' }
  if (sessionCookie) {
    headers.Cookie = `${SESSION_COOKIE_NAME}=${sessionCookie}`
  }

  const response = await fetch(GRAPHQL_URL, {
    method: 'POST',
    headers,
    body: JSON.stringify({ query, variables }),
  })

  const payload = await response.json()
  if (!response.ok || payload.errors?.length) {
    throw new Error(payload.errors?.[0]?.message || `GraphQL failed (${response.status})`)
  }

  const newSession = sessionCookieFromHeaders(response.headers)
  return {
    data: payload.data,
    sessionCookie: newSession || sessionCookie,
  }
}

async function requestSignInEmail(email) {
  const { data } = await nodeGraphqlRequest(REQUEST_SIGN_IN, { email })
  if (data.requestSignIn !== true) {
    throw new Error('requestSignIn returned false')
  }

  await pollForValue(() => latestLoginCodeFromLog(email), `sign-in email for ${email}`, { timeoutMs: 30000 })
}

async function applySessionCookie(page, sessionCookie) {
  const baseURL = process.env.BASE_URL || 'http://127.0.0.1:5173'
  const { hostname } = new URL(baseURL)

  await page.context().addCookies([
    {
      name: SESSION_COOKIE_NAME,
      value: sessionCookie,
      domain: hostname,
      path: '/',
      httpOnly: true,
      sameSite: 'Lax',
    },
  ])
}

export function setUserDisplayName(email, displayName, { avatarKey = 'compass' } = {}) {
  const normalizedEmail = email.trim().toLowerCase().replace(/'/g, "''")
  const escapedName = displayName.replace(/'/g, "''")
  const escapedAvatarKey = avatarKey.replace(/'/g, "''")
  const sql = `UPDATE users SET display_name='${escapedName}', avatar_key='${escapedAvatarKey}' WHERE email='${normalizedEmail}'`
  runPsql(sql)
}

async function expectSignedIn(page) {
  await expect(page.getByRole('button', { name: 'Log out' })).toBeVisible({ timeout: 15000 })
}

async function refreshSignedInUi(page) {
  await page.goto('/')
  await expectSignedIn(page)
}

export async function signInWithEmailLink(page, email) {
  await requestSignInEmail(email)

  const token = await pollForValue(() => latestMagicLinkTokenFromLog(email), `magic link for ${email}`, {
    timeoutMs: 15000,
  })
  expect(token).toBeTruthy()

  const { sessionCookie } = await nodeGraphqlRequest(COMPLETE_SIGN_IN_WITH_LINK, { token })
  if (!sessionCookie) {
    throw new Error('No session cookie returned from completeSignInWithLink')
  }

  await applySessionCookie(page, sessionCookie)
  await refreshSignedInUi(page)
}

export async function signInWithEmailCode(page, email) {
  await requestSignInEmail(email)

  const code = await pollForValue(() => latestLoginCodeFromLog(email), `sign-in code for ${email}`, {
    timeoutMs: 15000,
  })

  const { sessionCookie } = await nodeGraphqlRequest(COMPLETE_SIGN_IN_WITH_CODE, { email, code })
  if (!sessionCookie) {
    throw new Error('No session cookie returned from completeSignInWithCode')
  }

  await applySessionCookie(page, sessionCookie)
  await refreshSignedInUi(page)
}
