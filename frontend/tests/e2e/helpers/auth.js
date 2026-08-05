import { execSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect } from '@playwright/test'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..')
const E2E_BACKEND_LOG = path.join(repoRoot, 'tmp/e2e-backend.log')
const E2E_SIGNIN_LOG = process.env.E2E_SIGNIN_LOG_PATH
  ? path.isAbsolute(process.env.E2E_SIGNIN_LOG_PATH)
    ? process.env.E2E_SIGNIN_LOG_PATH
    : path.join(repoRoot, process.env.E2E_SIGNIN_LOG_PATH)
  : E2E_BACKEND_LOG
const GRAPHQL_URL = process.env.E2E_GRAPHQL_URL || 'http://127.0.0.1:8080/graphql'
const SESSION_COOKIE_NAME = 'lobby_session'
const CURL_TIMEOUT_SEC = 30

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

function readSignInLog() {
  for (const logPath of [E2E_SIGNIN_LOG, E2E_BACKEND_LOG]) {
    if (fs.existsSync(logPath)) {
      return fs.readFileSync(logPath, 'utf8')
    }
  }

  throw new Error(`E2E sign-in log not found at ${E2E_SIGNIN_LOG}`)
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
  const content = readSignInLog()

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
  const content = readSignInLog()

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

async function pollForValue(readValue, label, { timeoutMs = 15000 } = {}) {
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

function curlGraphql(query, variables = {}) {
  const payloadPath = path.join(repoRoot, 'tmp', `gql-${process.pid}-${Date.now()}.json`)
  fs.mkdirSync(path.dirname(payloadPath), { recursive: true })
  fs.writeFileSync(payloadPath, JSON.stringify({ query, variables }))

  try {
    return execSync(
      `curl -sS -i -m ${CURL_TIMEOUT_SEC} -X POST ${GRAPHQL_URL} -H 'Content-Type: application/json' --data-binary @${payloadPath}`,
      {
        encoding: 'utf8',
        maxBuffer: 10 * 1024 * 1024,
      },
    )
  } finally {
    fs.unlinkSync(payloadPath)
  }
}

function sessionCookieFromCurlOutput(output) {
  for (const line of output.split('\n')) {
    if (!line.toLowerCase().startsWith('set-cookie:')) {
      continue
    }

    const header = line.slice('set-cookie:'.length).trim()
    const [pair] = header.split(';')
    const eq = pair.indexOf('=')
    if (eq === -1) {
      continue
    }

    const name = pair.slice(0, eq).trim()
    if (name === SESSION_COOKIE_NAME) {
      return pair.slice(eq + 1)
    }
  }

  return null
}

function assertGraphqlSuccess(output) {
  const bodyIndex = output.indexOf('\r\n\r\n')
  const body = bodyIndex === -1 ? output : output.slice(bodyIndex + 4)
  const payload = JSON.parse(body)
  if (payload.errors?.length) {
    throw new Error(payload.errors[0]?.message || 'GraphQL request failed')
  }
  return payload.data
}

async function requestSignInEmail(email) {
  const output = curlGraphql(REQUEST_SIGN_IN, { email })
  const data = assertGraphqlSuccess(output)
  if (data.requestSignIn !== true) {
    throw new Error('requestSignIn returned false')
  }

  await pollForValue(() => latestLoginCodeFromLog(email), `sign-in email for ${email}`)
}

function completeSignInAndGetSessionCookie(query, variables) {
  const output = curlGraphql(query, variables)
  assertGraphqlSuccess(output)
  const sessionCookie = sessionCookieFromCurlOutput(output)
  if (!sessionCookie) {
    throw new Error('No session cookie returned from sign-in completion')
  }
  return sessionCookie
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
  await expect(page.getByRole('button', { name: 'Log out' })).toBeVisible({ timeout: 30000 })
}

async function refreshSignedInUi(page) {
  await page.goto('/')
  await expectSignedIn(page)
}

export async function signInWithEmailLink(page, email) {
  await requestSignInEmail(email)

  const token = await pollForValue(() => latestMagicLinkTokenFromLog(email), `magic link for ${email}`)
  const sessionCookie = completeSignInAndGetSessionCookie(COMPLETE_SIGN_IN_WITH_LINK, { token })

  await applySessionCookie(page, sessionCookie)
  await refreshSignedInUi(page)
}

export async function signInWithEmailCode(page, email) {
  await requestSignInEmail(email)

  const code = await pollForValue(() => latestLoginCodeFromLog(email), `sign-in code for ${email}`)
  const sessionCookie = completeSignInAndGetSessionCookie(COMPLETE_SIGN_IN_WITH_CODE, { email, code })

  await applySessionCookie(page, sessionCookie)
  await refreshSignedInUi(page)
}
