import { test, expect } from '@playwright/test'
import { setUserDisplayName, signInWithEmailCode } from './helpers/auth.js'
import {
  clearDemoMatchmakingState,
  restorePrimaryGameHandoffUrls,
  setPrimaryGameAPIBaseUrl,
} from './helpers/db.js'
import { startMockGameServer, stopMockGameServer } from './helpers/mockGame.js'
import {
  enterMatch,
  expectAutoLaunch,
  expectLaunchStep,
  expectNoIntentBanner,
  expectWaitingPage,
  joinDemoGameQueue,
  returnFromMatch,
} from './helpers/queue.js'

/**
 * Signing in leaves display_name and avatar null — the identity prompt is what
 * fills them in. Play entry requires both (JQ-126), so a fixture that queues has
 * to finish that step the way a real player would before reaching a queue.
 */
async function identifyPlayer(page, email, displayName) {
  setUserDisplayName(email, displayName)
  await page.reload()
}

test.describe('Game loop', () => {
  /** @type {import('node:http').Server | undefined} */
  let mockServer

  test.beforeAll(async () => {
    const mock = await startMockGameServer()
    mockServer = mock.server
    setPrimaryGameAPIBaseUrl(mock.baseUrl)
  })

  test.afterAll(async () => {
    restorePrimaryGameHandoffUrls()
    if (mockServer) {
      await stopMockGameServer(mockServer)
    }
  })

  test.beforeEach(() => {
    clearDemoMatchmakingState()
  })

  // Two matches, and each one now spends the launch countdown before the player is
  // carried into the game (JQ-136). That does not fit the config's local 30s budget.
  test('match, return home, and re-queue with a fresh session', async ({ browser }) => {
    test.setTimeout(90_000)

    const emailA = `loop-a-${Date.now()}@example.com`
    const emailB = `loop-b-${Date.now()}@example.com`

    const contextA = await browser.newContext()
    const contextB = await browser.newContext()
    const pageA = await contextA.newPage()
    const pageB = await contextB.newPage()

    try {
      await pageA.goto('/')
      await signInWithEmailCode(pageA, emailA)
      await identifyPlayer(pageA, emailA, 'Loop A')
      await pageB.goto('/')
      await signInWithEmailCode(pageB, emailB)
      await identifyPlayer(pageB, emailB, 'Loop B')

      await joinDemoGameQueue(pageA)
      await expectWaitingPage(pageA)

      await joinDemoGameQueue(pageB)
      // A was on the waiting page when the match formed, so the launch moment is
      // its surface. Nothing is clicked: the countdown is what carries A in.
      await expectLaunchStep(pageA)
      const firstMatchId = await expectAutoLaunch(pageA)
      expect(await enterMatch(pageB)).toBe(firstMatchId)

      await returnFromMatch(pageA, firstMatchId)
      await expectNoIntentBanner(pageA)

      await returnFromMatch(pageB, firstMatchId)
      await expectNoIntentBanner(pageB)

      await joinDemoGameQueue(pageA)
      await expectWaitingPage(pageA)
      await joinDemoGameQueue(pageB)
      await expectLaunchStep(pageA)
      const secondMatchId = await expectAutoLaunch(pageA)
      expect(secondMatchId).not.toBe(firstMatchId)
      expect(await enterMatch(pageB)).toBe(secondMatchId)
    } finally {
      await contextA.close()
      await contextB.close()
    }
  })
})
