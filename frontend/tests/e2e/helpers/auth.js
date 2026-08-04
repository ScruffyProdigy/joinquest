import { execSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect } from '@playwright/test'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..')
const E2E_BACKEND_LOG = path.join(repoRoot, 'tmp/e2e-backend.log')

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

async function graphqlRequest(page, query, variables = {}) {
  return page.evaluate(
    async ({ query: gqlQuery, variables: gqlVariables }) => {
      const response = await fetch('/graphql', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ query: gqlQuery, variables: gqlVariables }),
      })

      const payload = await response.json()
      if (!response.ok) {
        throw new Error(payload.errors?.[0]?.message || `API request failed (${response.status})`)
      }
      if (payload.errors?.length) {
        throw new Error(payload.errors[0]?.message || 'GraphQL request failed')
      }

      return payload.data
    },
    { query, variables },
  )
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

async function submitSignInEmail(page, email) {
  const emailInput = page.getByRole('textbox', { name: 'Email' })
  await expect(emailInput).toBeVisible({ timeout: 15000 })
  await expect(emailInput).toBeEnabled()
  await emailInput.fill(email)
  await page.getByRole('button', { name: 'Continue with email' }).click()
  await expect(page.getByRole('heading', { name: 'Enter your code' })).toBeVisible({ timeout: 20000 })
  await pollForValue(() => latestLoginCodeFromLog(email), `sign-in email for ${email}`, { timeoutMs: 20000 })
}

async function refreshSignedInUi(page) {
  await page.goto('/')
  await expectSignedIn(page)
}

export async function signInWithEmailLink(page, email) {
  await submitSignInEmail(page, email)

  const token = await pollForValue(() => latestMagicLinkTokenFromLog(email), `magic link for ${email}`, {
    timeoutMs: 15000,
  })
  expect(token).toBeTruthy()

  await graphqlRequest(page, COMPLETE_SIGN_IN_WITH_LINK, { token })
  await refreshSignedInUi(page)
}

export async function signInWithEmailCode(page, email) {
  await submitSignInEmail(page, email)

  const code = await pollForValue(() => latestLoginCodeFromLog(email), `sign-in code for ${email}`, {
    timeoutMs: 15000,
  })

  await graphqlRequest(page, COMPLETE_SIGN_IN_WITH_CODE, { email, code })
  await refreshSignedInUi(page)
}
