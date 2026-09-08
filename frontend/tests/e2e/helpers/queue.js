import { expect } from '@playwright/test'
import {
  FINDING_PLAYERS,
  FIND_A_GAME_HEADING,
  RESULTS_LEAVE_MATCH,
  RESULTS_STILL_PLAYING,
  RESULTS_YOUR_RESULT_SO_FAR,
} from '../../../src/lib/playerCopy.js'

// Seed row ...0001 (JQ-203). It is the only seeded card with a mode queue
// outside production, so E2E matchmaking runs through it; its handoff URL is
// repointed at the local demo game server by helpers/db.js.
const DEMO_GAME_NAME = 'Word Hunt'

function demoGameCardLink(page) {
  return page.getByRole('link').filter({
    has: page.getByRole('heading', { name: DEMO_GAME_NAME }),
  })
}

export async function joinDemoGameQueue(page) {
  await demoGameCardLink(page).click()
  await expect(page.getByRole('heading', { name: DEMO_GAME_NAME, level: 1 })).toBeVisible()
  await page.getByRole('button', { name: 'Look for group' }).click()
}

// The queued state is its own page now (JQ-197), not a banner over the catalog.
export async function expectWaitingPage(page) {
  await expect(page).toHaveURL(/\/waiting\/?$/)
  const card = page.getByRole('region', { name: 'Looking for a group' })
  await expect(card).toBeVisible()
  await expect(card).toContainText(FINDING_PLAYERS)
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

/**
 * Walk the post-match screen the way a player does (JQ-135).
 *
 * `/return?match=…` no longer bounces the player home on its own — it renders the
 * post-match screen, and leaving is an explicit action. No game in these fixtures reports
 * a finish or a result, so the session is still `active` when the browser arrives:
 * `MatchResult.complete` is false and ReturnPage takes its still-playing branch ("Your
 * result so far" + "Still playing" + the exit button). That branch is asserted rather than
 * clicked straight through, so a regression that stops rendering the screen fails here
 * instead of silently falling back to the old redirect.
 *
 * Landing home is still the assertion that matters: it is what proves the return released
 * the player's matched queue row and left them free to queue again.
 */
export async function returnFromMatch(page, matchId) {
  await page.goto(`/return?match=${encodeURIComponent(matchId)}`)

  await expect(page.getByText(RESULTS_YOUR_RESULT_SO_FAR)).toBeVisible({ timeout: 20000 })
  await expect(page.getByText(RESULTS_STILL_PLAYING)).toBeVisible()

  await page.getByRole('button', { name: RESULTS_LEAVE_MATCH }).click()

  await expect(page.getByRole('heading', { level: 1, name: FIND_A_GAME_HEADING })).toBeVisible({
    timeout: 20000,
  })
}
