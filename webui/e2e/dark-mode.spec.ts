import { test, expect } from '@playwright/test'

test.describe('Dark Mode', () => {
  test.use({ colorScheme: 'dark' })

  test('User Login - dark mode', async ({ page }) => {
    await page.goto('/')
    await page.waitForSelector('text=HermesX')
    await page.screenshot({ path: 'e2e/screenshots/dark-user-login.png', fullPage: true })
  })

  test('Admin Login - dark mode', async ({ page }) => {
    await page.goto('/admin.html')
    await page.waitForSelector('text=HermesX')
    await page.screenshot({ path: 'e2e/screenshots/dark-admin-login.png', fullPage: true })
  })

  test('Admin Bootstrap - dark mode', async ({ page }) => {
    await page.goto('/admin.html#/bootstrap')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/dark-admin-bootstrap.png', fullPage: true })
  })
})
