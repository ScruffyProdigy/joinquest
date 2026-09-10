import { test, expect } from '@playwright/test'

test.describe('Debug Tests', () => {
  test('server responds', async ({ page }) => {
    try {
      console.log('Navigating to page...')
      await page.goto('/', { timeout: 60000 })
      
      console.log('Waiting for page to load...')
      await page.waitForLoadState('domcontentloaded', { timeout: 30000 })
      
      console.log('Getting page content...')
      const content = await page.content()
      console.log('Page content length:', content.length)
      console.log('Page content preview:', content.substring(0, 500))
      
      console.log('Checking if the catalog heading exists...')
      const hasCatalog = await page.getByRole('heading', { level: 1, name: 'Solo or squad, just join.' }).isVisible()
      console.log('Catalog heading visible:', hasCatalog)
      
      console.log('Checking page title...')
      const title = await page.title()
      console.log('Page title:', title)
      
      console.log('Checking window.env...')
      const envConfig = await page.evaluate(() => {
        return {
          hasWindowEnv: typeof window.env !== 'undefined',
          windowEnv: window.env
        }
      })
      console.log('Window.env check:', envConfig)
      
      // Basic assertions
      expect(content.length).toBeGreaterThan(0)
      expect(title).toBeTruthy()
      
    } catch (error) {
      console.error('Test failed:', error)
      await page.screenshot({ path: 'debug-test-failure.png' })
      throw error
    }
  })
})
