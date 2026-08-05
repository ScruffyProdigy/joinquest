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
const SIGNED_IN_TIMEOUT_MS = 90000

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

function latestMagicLinkUrlFromLog(email) {
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

  return lastLink
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

async function submitEmailForSignIn(page, email) {
  await page.getByRole('textbox', { name: 'Email' }).fill(email)
  await page.getByRole('button', { name: 'Continue with email' }).click()
  await expect(page.getByRole('heading', { name: 'Enter your code' })).toBeVisible({ timeout: 30000 })
  await pollForValue(() => latestLoginCodeFromLog(email), `sign-in email for ${email}`)
}

async function expectSignedIn(page) {
  await expect(page.getByRole('button', { name: 'Log out' })).toBeVisible({ timeout: SIGNED_IN_TIMEOUT_MS })
}

export function setUserDisplayName(email, displayName, { avatarKey = 'compass' } = {}) {
  const normalizedEmail = email.trim().toLowerCase().replace(/'/g, "''")
  const escapedName = displayName.replace(/'/g, "''")
  const escapedAvatarKey = avatarKey.replace(/'/g, "''")
  const sql = `UPDATE users SET display_name='${escapedName}', avatar_key='${escapedAvatarKey}' WHERE email='${normalizedEmail}'`
  runPsql(sql)
}

export async function signInWithEmailLink(page, email) {
  await submitEmailForSignIn(page, email)

  const magicLink = await pollForValue(() => latestMagicLinkUrlFromLog(email), `magic link for ${email}`)
  await page.goto(magicLink)
  await page.waitForURL((url) => !url.pathname.includes('/auth/complete'), { timeout: SIGNED_IN_TIMEOUT_MS })
  await expectSignedIn(page)
}

export async function signInWithEmailCode(page, email) {
  await submitEmailForSignIn(page, email)

  const code = await pollForValue(() => latestLoginCodeFromLog(email), `sign-in code for ${email}`)
  await page.getByLabel('Sign-in code').fill(code)
  await expectSignedIn(page)
}
