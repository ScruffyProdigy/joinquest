import { test, expect } from '@playwright/test'
import { SIGN_IN_DIALOG_TITLE, SIGN_IN_OR_JOIN } from '../../src/lib/playerCopy.js'
import { setUserDisplayName, signInWithEmailCode, signInWithEmailLink } from './helpers/auth.js'

/** Home leads with the catalog now, so sign-in lives behind the header pill. */
async function openSignIn(page) {
  await page.goto('/')
  await page.getByRole('button', { name: SIGN_IN_OR_JOIN }).click()
  await expect(page.getByRole('heading', { name: SIGN_IN_DIALOG_TITLE })).toBeVisible()
}

/**
 * The session card on /account. Scoped because the account page also lists the
 * same email under "Email addresses", so an unscoped match is ambiguous.
 */
function sessionCard(page, heading) {
  return page.getByLabel(heading)
}

/** The session card lives on /account now, so go there to inspect it. */
async function openAccount(page) {
  await page.goto('/account')
}

/**
 * Log out from /account. The starter avatars load after the card renders and
 * shift it downwards, so wait for them before clicking or the click can land
 * where the button used to be.
 */
async function logOut(page) {
  await expect(page.getByRole('button', { name: 'Compass' })).toBeVisible()
  await page.getByRole('button', { name: 'Log out' }).click()
}

/** What /account shows once nobody is signed in. */
async function expectSignedOut(page) {
  await expect(page.getByText('Sign in to manage your account.')).toBeVisible()
}

test.describe('Auth flow', () => {
  test('signs in with a magic link and logs out', async ({ page }) => {
    const email = `e2e-${Date.now()}@example.com`

    await openSignIn(page)

    await signInWithEmailLink(page, email)
    await openAccount(page)

    await expect(page.getByRole('heading', { name: 'Set up your display' })).toBeVisible()
    await expect(sessionCard(page, 'Set up your display').getByText(email)).toBeVisible()
    // Nothing invents a name any more, so the field starts empty.
    await expect(page.getByLabel('Display name')).toHaveValue('')

    await logOut(page)
    await expectSignedOut(page)
    await expect(page.getByText(email)).not.toBeVisible()
  })

  test('signs in with a 6-digit email code', async ({ page }) => {
    const email = `e2e-code-${Date.now()}@example.com`

    await openSignIn(page)
    await signInWithEmailCode(page, email)
    await openAccount(page)

    await expect(page.getByRole('heading', { name: 'Set up your display' })).toBeVisible()
    await expect(sessionCard(page, 'Set up your display').getByText(email)).toBeVisible()
    await expect(page.getByLabel('Display name')).toHaveValue('')
  })

  test('keeps an existing display name when a returning user signs in again', async ({ page }) => {
    const email = `returning-${Date.now()}@example.com`
    const customDisplayName = 'Returning Player'

    await openSignIn(page)
    await signInWithEmailLink(page, email)
    await openAccount(page)

    setUserDisplayName(email, customDisplayName)

    await logOut(page)
    await expectSignedOut(page)

    await signInWithEmailLink(page, email)
    await openAccount(page)

    await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible()
    await expect(page.getByText(customDisplayName)).toBeVisible()
  })
})
