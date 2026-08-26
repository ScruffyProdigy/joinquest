import { test, expect } from '@playwright/test'

test.describe('Basic App Functionality', () => {
  test('app loads and displays content', async ({ page }) => {
    try {
      // Navigate to the app with a longer timeout
      await page.goto('/', { timeout: 60000 })
      
      // Wait for the page to load with multiple strategies
      await page.waitForLoadState('domcontentloaded', { timeout: 30000 })
      await page.waitForLoadState('networkidle', { timeout: 30000 })
      
      // Check that the basic content is present
      await expect(page.getByRole('heading', { level: 1, name: 'Find a game', includeHidden: true })).toBeVisible({ timeout: 10000 })
    } catch (error) {
      // If the test fails, take a screenshot for debugging
      await page.screenshot({ path: 'test-failure.png' })
      throw error
    }
  })

  test('page has correct title', async ({ page }) => {
    try {
      await page.goto('/', { timeout: 60000 })
      await page.waitForLoadState('domcontentloaded', { timeout: 30000 })
      
      // Check the page title
      await expect(page).toHaveTitle(/JoinQuest/, { timeout: 10000 })
    } catch (error) {
      await page.screenshot({ path: 'title-test-failure.png' })
      throw error
    }
  })

  test('environment configuration is available', async ({ page }) => {
    try {
      await page.goto('/', { timeout: 60000 })
      await page.waitForLoadState('domcontentloaded', { timeout: 30000 })
      
      // Check that window.env is available
      const envConfig = await page.evaluate(() => {
        return typeof window.env !== 'undefined' ? window.env : null
      })
      
      expect(envConfig).toBeTruthy()
      expect(envConfig).toHaveProperty('REACT_APP_ENV')
      expect(envConfig).toHaveProperty('REACT_APP_API_BASE_URL')
    } catch (error) {
      await page.screenshot({ path: 'env-test-failure.png' })
      throw error
    }
  })
})
