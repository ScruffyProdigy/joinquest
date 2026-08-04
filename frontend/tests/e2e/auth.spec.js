import { test, expect } from '@playwright/test'
import { SIGN_IN_HEADING } from '../../src/lib/playerCopy.js'
import { setUserDisplayName, signInWithEmailCode, signInWithEmailLink } from './helpers/auth.js'

test.describe('Auth flow', () => {
  test('signs in with a magic link and logs out', async ({ page }) => {
    const email = `e2e-${Date.now()}@example.com`

    await page.goto('/')
    await expect(page.getByRole('heading', { name: SIGN_IN_HEADING })).toBeVisible()

    await signInWithEmailLink(page, email)

    await expect(page.getByRole('heading', { name: 'Set up your display' })).toBeVisible()
    await expect(page.getByText(email)).toBeVisible()
    await expect(page.getByLabel('Display name')).toHaveValue(email.split('@')[0])

    await page.getByRole('button', { name: 'Log out' }).click()
    await expect(page.getByRole('heading', { name: SIGN_IN_HEADING })).toBeVisible()
    await expect(page.getByText(email)).not.toBeVisible()
  })

  test('signs in with a 6-digit email code', async ({ page }) => {
    const email = `e2e-code-${Date.now()}@example.com`

    await page.goto('/')
    await signInWithEmailCode(page, email)

    await expect(page.getByRole('heading', { name: 'Set up your display' })).toBeVisible()
    await expect(page.getByText(email)).toBeVisible()
    await expect(page.getByLabel('Display name')).toHaveValue(email.split('@')[0])
  })

  test('keeps an existing display name when a returning user signs in again', async ({ page }) => {
    const email = `returning-${Date.now()}@example.com`
    const customDisplayName = 'Returning Player'

    await page.goto('/')
    await signInWithEmailLink(page, email)

    setUserDisplayName(email, customDisplayName)

    await page.getByRole('button', { name: 'Log out' }).click()
    await expect(page.getByRole('heading', { name: SIGN_IN_HEADING })).toBeVisible()

    await signInWithEmailLink(page, email)

    await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible()
    await expect(page.getByText(customDisplayName)).toBeVisible()
  })
})
