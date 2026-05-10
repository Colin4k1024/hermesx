import { test, expect } from '@playwright/test'

test.describe('User Portal', () => {
  test('Login page renders correctly', async ({ page }) => {
    await page.goto('/')
    await page.waitForSelector('text=HermesX')
    await page.screenshot({ path: 'e2e/screenshots/user-login.png', fullPage: true })
    await expect(page.locator('text=Connect to your AI agent')).toBeVisible()
  })

  test('Login with API key and access Chat', async ({ page }) => {
    await page.goto('/')
    await page.waitForSelector('input[type="password"]')
    
    // Try connecting with a test key
    const input = page.locator('input[type="password"]')
    await input.fill('hx-test-key-for-screenshots')
    await page.screenshot({ path: 'e2e/screenshots/user-login-filled.png', fullPage: true })
    
    await page.locator('button:has-text("Connect")').click()
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/user-after-login.png', fullPage: true })
  })

  test('Chat page with session sidebar', async ({ page }) => {
    // Directly set auth state and navigate
    await page.goto('/')
    await page.evaluate(() => {
      sessionStorage.setItem('hermesx_user_meta', JSON.stringify({ endpoint: '' }))
    })
    // Use the auth store to connect
    await page.goto('/#/chat')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/user-chat.png', fullPage: true })
  })

  test('Memories page', async ({ page }) => {
    await page.goto('/#/memories')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/user-memories.png', fullPage: true })
  })

  test('Skills page', async ({ page }) => {
    await page.goto('/#/skills')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/user-skills.png', fullPage: true })
  })

  test('Usage page', async ({ page }) => {
    await page.goto('/#/usage')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/user-usage.png', fullPage: true })
  })

  test('Settings page', async ({ page }) => {
    await page.goto('/#/settings')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/user-settings.png', fullPage: true })
  })

  test('Notifications page', async ({ page }) => {
    await page.goto('/#/notifications')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/user-notifications.png', fullPage: true })
  })
})

test.describe('Admin Console', () => {
  test('Admin Login page renders correctly', async ({ page }) => {
    await page.goto('/admin.html')
    await page.waitForSelector('text=HermesX')
    await page.screenshot({ path: 'e2e/screenshots/admin-login.png', fullPage: true })
  })

  test('Admin Login with key and access Dashboard', async ({ page }) => {
    await page.goto('/admin.html')
    await page.waitForSelector('input[type="password"]')
    
    const input = page.locator('input[type="password"]')
    await input.fill('hx-admin-test-key')
    await page.locator('button:has-text("Sign In")').click()
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/admin-after-login.png', fullPage: true })
  })

  test('Admin Bootstrap page', async ({ page }) => {
    await page.goto('/admin.html#/bootstrap')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-bootstrap.png', fullPage: true })
  })

  test('Admin Dashboard page', async ({ page }) => {
    await page.goto('/admin.html#/dashboard')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-dashboard.png', fullPage: true })
  })

  test('Admin Tenants page', async ({ page }) => {
    await page.goto('/admin.html#/tenants')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-tenants.png', fullPage: true })
  })

  test('Admin Users page', async ({ page }) => {
    await page.goto('/admin.html#/users')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-users.png', fullPage: true })
  })

  test('Admin API Keys page', async ({ page }) => {
    await page.goto('/admin.html#/keys')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-apikeys.png', fullPage: true })
  })

  test('Admin Audit Logs page', async ({ page }) => {
    await page.goto('/admin.html#/audit')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-audit.png', fullPage: true })
  })

  test('Admin Pricing page', async ({ page }) => {
    await page.goto('/admin.html#/pricing')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-pricing.png', fullPage: true })
  })

  test('Admin Sandbox page', async ({ page }) => {
    await page.goto('/admin.html#/sandbox')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-sandbox.png', fullPage: true })
  })

  test('Admin System Settings page', async ({ page }) => {
    await page.goto('/admin.html#/settings')
    await page.waitForTimeout(1000)
    await page.screenshot({ path: 'e2e/screenshots/admin-settings.png', fullPage: true })
  })
})

test.describe('Responsive - Mobile', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test('User Login - mobile', async ({ page }) => {
    await page.goto('/')
    await page.waitForSelector('text=HermesX')
    await page.screenshot({ path: 'e2e/screenshots/mobile-user-login.png', fullPage: true })
  })

  test('Admin Login - mobile', async ({ page }) => {
    await page.goto('/admin.html')
    await page.waitForSelector('text=HermesX')
    await page.screenshot({ path: 'e2e/screenshots/mobile-admin-login.png', fullPage: true })
  })
})
