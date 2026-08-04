import { execSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect } from '@playwright/test'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..')
const E2E_BACKEND_LOG = path.join(repoRoot, 'tmp/e2e-backend.log')

function psqlAvailable() {
  try {
    execSync('command -v psql', { stdio: 'ignore' })
    return true
  } catch {
    return false
  }
}

function queryMagicLinkToken(sql) {
  if (process.env.DATABASE_URL && psqlAvailable()) {
    return execSync(`psql "${process.env.DATABASE_URL}" -tAc "${sql}"`, {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    }).trim()
  }

  return execSync(
    `docker compose exec -T postgres psql -U app -d playhub -tAc "${sql}"`,
    {
      cwd: repoRoot,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    },
  ).trim()
}

export function latestMagicLinkToken(email) {
  const normalizedEmail = email.trim().toLowerCase().replace(/'/g, "''")
  const sql = `SELECT token FROM magic_links WHERE email='${normalizedEmail}' ORDER BY created_at DESC LIMIT 1`
  return queryMagicLinkToken(sql)
}

function latestLoginCodeFromLog(email) {
  if (!fs.existsSync(E2E_BACKEND_LOG)) {
    throw new Error(`E2E backend log not found at ${E2E_BACKEND_LOG}`)
  }

  const normalizedEmail = email.trim().toLowerCase()
  const escapedEmail = normalizedEmail.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const pattern = new RegExp(`email: sign-in for ${escapedEmail} code=(\\d{6})`, 'g')
  const content = fs.readFileSync(E2E_BACKEND_LOG, 'utf8')

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

async function pollForLoginCode(email, { timeoutMs = 10000 } = {}) {
  const deadline = Date.now() + timeoutMs

  while (Date.now() < deadline) {
    try {
      return latestLoginCodeFromLog(email)
    } catch {
      await new Promise((resolve) => {
        setTimeout(resolve, 200)
      })
    }
  }

  throw new Error(`Timed out waiting for sign-in code for ${email}`)
}

export function setUserDisplayName(email, displayName) {
  const normalizedEmail = email.trim().toLowerCase().replace(/'/g, "''")
  const escapedName = displayName.replace(/'/g, "''")
  const sql = `UPDATE users SET display_name='${escapedName}' WHERE email='${normalizedEmail}'`
  queryMagicLinkToken(sql)
}

export async function signInWithEmailLink(page, email) {
  await page.getByRole('textbox', { name: 'Email' }).fill(email)
  await page.getByRole('button', { name: 'Continue' }).click()
  await expect(page.getByRole('heading', { name: 'Enter your code' })).toBeVisible()

  const token = latestMagicLinkToken(email)
  expect(token).toBeTruthy()

  await page.goto(`/auth/complete?token=${encodeURIComponent(token)}`)
  await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible({ timeout: 15000 })
}

export async function signInWithEmailCode(page, email) {
  await page.getByRole('textbox', { name: 'Email' }).fill(email)
  await page.getByRole('button', { name: 'Continue' }).click()
  await expect(page.getByRole('heading', { name: 'Enter your code' })).toBeVisible()

  const code = await pollForLoginCode(email)
  await page.getByLabel('Sign-in code').fill(code)
  await page.getByRole('button', { name: 'Continue' }).click()
  await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible({ timeout: 15000 })
}
