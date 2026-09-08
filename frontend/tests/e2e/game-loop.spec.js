import { test, expect } from '@playwright/test'
import { setUserDisplayName, signInWithEmailCode } from './helpers/auth.js'
import {
  clearDemoMatchmakingState,
  restorePrimaryGameHandoffUrls,
  setPrimaryGameAPIBaseUrl,
} from './helpers/db.js'
import { startMockGameServer, stopMockGameServer } from './helpers/mockGame.js'
import {
  expectMatchedBanner,
  expectNoIntentBanner,
  expectWaitingBanner,
  joinDemoGameQueue,
  readLaunchMatchId,
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

  test('match, return home, and re-queue with a fresh session', async ({ browser }) => {
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
      await expectWaitingBanner(pageA)

      await joinDemoGameQueue(pageB)
      await expectMatchedBanner(pageA)
      await expectMatchedBanner(pageB)

      const firstMatchId = await readLaunchMatchId(pageA)
      expect(await readLaunchMatchId(pageB)).toBe(firstMatchId)

      await returnFromMatch(pageA, firstMatchId)
      await expectNoIntentBanner(pageA)

      await returnFromMatch(pageB, firstMatchId)
      await expectNoIntentBanner(pageB)

      await joinDemoGameQueue(pageA)
      await expectWaitingBanner(pageA)
      await joinDemoGameQueue(pageB)
      await expectMatchedBanner(pageA)
      await expectMatchedBanner(pageB)

      const secondMatchId = await readLaunchMatchId(pageA)
      expect(secondMatchId).not.toBe(firstMatchId)
      expect(await readLaunchMatchId(pageB)).toBe(secondMatchId)
    } finally {
      await contextA.close()
      await contextB.close()
    }
  })
})
