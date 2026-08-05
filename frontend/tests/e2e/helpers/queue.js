import { expect } from '@playwright/test'

const RPS_GAME_NAME = 'Rock Paper Scissors Lizard Robot'

function rpsGameRow(page) {
  return page.locator('li.game-list-item').filter({
    has: page.getByRole('heading', { name: RPS_GAME_NAME }),
  })
}

export async function joinRockPaperQueue(page) {
  const row = rpsGameRow(page)
  await row.getByRole('button', { name: 'Look for group' }).click()
}

export async function expectWaitingBanner(page) {
  const banner = page.getByRole('region', { name: 'Your intent' })
  await expect(banner).toBeVisible()
  await expect(banner).toContainText('Looking for a group')
}

export async function expectMatchedBanner(page) {
  const banner = page.getByRole('region', { name: 'Your intent' })
  await expect(banner).toBeVisible({ timeout: 20000 })
  await expect(banner).toContainText('Playing')
}

export async function expectNoIntentBanner(page) {
  await expect(page.getByRole('region', { name: 'Your intent' })).not.toBeVisible()
}
export async function readLaunchMatchId(page) {
  const banner = page.getByRole('region', { name: 'Your intent' })
  const launchLink = banner.getByRole('link', {
    name: 'Launch game',
  })
  await expect(launchLink).toBeVisible({ timeout: 30000 })
  const href = await launchLink.getAttribute('href')
  if (!href) {
    throw new Error('Launch game link is missing href')
  }
  const matchId = new URL(href).searchParams.get('match')
  if (!matchId) {
    throw new Error(`Launch URL missing match param: ${href}`)
  }
  return matchId
}

export async function returnFromMatch(page, matchId) {
  await page.goto(`/return?match=${encodeURIComponent(matchId)}`)
  await expect(page.getByRole('heading', { name: 'Available games' })).toBeVisible({
    timeout: 20000,
  })
}
