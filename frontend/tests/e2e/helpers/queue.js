import { expect } from '@playwright/test'
import {
  FINDING_PLAYERS,
  WAITING_REGION_LABEL,
  APP_TAGLINE,
  MATCH_IN_PROGRESS,
  READY_TO_LAUNCH,
  REJOIN_MATCH,
  RESULTS_IN_PROGRESS_TITLE,
  RESULTS_LEAVE_MATCH,
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
  await page.getByRole('button', { name: 'Jump in' }).click()
}

// The queued state is its own page now (JQ-197), not a banner over the catalog.
export async function expectWaitingPage(page) {
  await expect(page).toHaveURL(/\/waiting\/?$/)
  const card = page.getByRole('region', { name: WAITING_REGION_LABEL })
  await expect(card).toBeVisible()
  await expect(card).toContainText(FINDING_PLAYERS)
}

// A player who was on the waiting page when the match formed gets the launch step
// (JQ-136), not the banner: the countdown carries them into the game on its own.
export async function expectLaunchStep(page) {
  const card = page.getByRole('region', { name: 'Match found' })
  await expect(card).toBeVisible({ timeout: 20000 })
  await expect(card).toContainText(READY_TO_LAUNCH)
}

const LAUNCHED_URL = /\/return\?match=/

function matchIdFromLaunchUrl(url) {
  const matchId = new URL(url).searchParams.get('match')
  if (!matchId) {
    throw new Error(`Launch URL missing match param: ${url}`)
  }
  return matchId
}

/** Waits out the countdown and reports the match the player was carried into. */
export async function expectAutoLaunch(page) {
  await expect(page).toHaveURL(LAUNCHED_URL, { timeout: 30000 })
  return matchIdFromLaunchUrl(page.url())
}

/**
 * Either way into the match, so the walk does not depend on how fast the queue
 * resolved: a player who was waiting is carried in by the countdown (JQ-136), and one
 * who matched somewhere else in the lobby meets the match dialog (JQ-261), whose own
 * countdown carries them in. Rejoin is clicked only if the countdown has not already.
 */
export async function enterMatch(page) {
  const rejoin = activeMatchDialog(page).getByRole('button', { name: REJOIN_MATCH })

  await expect(async () => {
    if (!LAUNCHED_URL.test(page.url())) {
      if (await rejoin.isVisible()) {
        await rejoin.click()
      }
      expect(page.url()).toMatch(LAUNCHED_URL)
    }
  }).toPass({ timeout: 30000 })

  return matchIdFromLaunchUrl(page.url())
}

function activeMatchDialog(page) {
  return page.getByRole('region', { name: MATCH_IN_PROGRESS })
}

/**
 * The lobby has let go of the match. Worth asserting on its own: the dialog is
 * unconditional and auto-rejoins, so an intent the server failed to release would not
 * merely linger the way the old banner did -- it would carry the player back in.
 */
export async function expectNoActiveMatchDialog(page) {
  await expect(activeMatchDialog(page)).not.toBeVisible()
}

/**
 * Walk the post-match screen the way a player does (JQ-135).
 *
 * `/return?match=…` no longer bounces the player home on its own — it renders the
 * post-match screen, and leaving is an explicit action. No game in these fixtures reports
 * a finish or a result, and `AcknowledgePlayerReturn` does not complete a session on its
 * own, so the session is still `active` when either browser arrives: `MatchResult.complete`
 * is false, the standings card titles itself "Results so far", and the exit button is the
 * only action offered. That is asserted rather than clicked straight through, so a
 * regression that stops rendering the screen fails here instead of silently falling back to
 * the old redirect.
 *
 * The title is the marker, not "Still playing" (JQ-277). That is now a per-row badge on the
 * standings rather than a card of its own, and it is only on rows that have not finished —
 * so it is there for the first player to return and gone for the second, by which point
 * both are marked finished.
 *
 * Landing home is still the assertion that matters: it is what proves the return released
 * the player's matched queue row and left them free to queue again.
 */
export async function returnFromMatch(page, matchId) {
  await page.goto(`/return?match=${encodeURIComponent(matchId)}`)

  await expect(page.getByText(RESULTS_IN_PROGRESS_TITLE)).toBeVisible({ timeout: 20000 })

  await page.getByRole('button', { name: RESULTS_LEAVE_MATCH }).click()

  await expect(page.getByRole('heading', { level: 1, name: APP_TAGLINE })).toBeVisible({
    timeout: 20000,
  })
}
