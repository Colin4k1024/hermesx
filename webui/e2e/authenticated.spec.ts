import { test } from '@playwright/test'
import type { Page } from '@playwright/test'

async function mockAuthAndLogin(page: Page, type: 'user' | 'admin') {
  // Intercept /v1/me to return a successful auth response
  await page.route('**/v1/me', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        tenant_id: 'default',
        identity: type === 'admin' ? 'admin' : 'demo-user',
        roles: type === 'admin' ? ['admin'] : ['user'],
        auth_method: 'api_key',
        plan: 'pro',
        rate_limit_rpm: 60,
        max_sessions: 100,
      }),
    })
  })

  // Mock tenant list (hook calls /v1/tenants with asAdmin header)
  await page.route('**/v1/tenants', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        tenants: [
          { id: 'tenant-1', name: 'Acme Corp', created_at: '2025-01-01T00:00:00Z' },
          { id: 'tenant-2', name: 'Beta Inc', created_at: '2025-02-15T00:00:00Z' },
          { id: 'default', name: 'Default Tenant', created_at: '2024-12-01T00:00:00Z' },
        ],
        total: 3,
      }),
    })
  })

  // Mock sessions for chat page — matches SessionListResponse
  await page.route('**/v1/sessions*', async (route) => {
    if (route.request().url().includes('/v1/sessions/')) {
      // Single session detail
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          session_id: 'sess-1',
          messages: [
            { role: 'user', content: 'Hello!', timestamp: '2025-05-10T10:00:00Z' },
            { role: 'assistant', content: 'Hi! How can I help you today?', timestamp: '2025-05-10T10:00:01Z' },
          ],
        }),
      })
    } else {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          tenant_id: 'default',
          user_id: 'demo-user',
          sessions: [
            { id: 'sess-1', started_at: '2025-05-10T10:00:00Z', ended_at: null, message_count: 4 },
            { id: 'sess-2', started_at: '2025-05-09T08:00:00Z', ended_at: '2025-05-09T09:30:00Z', message_count: 12 },
          ],
          count: 2,
        }),
      })
    }
  })

  // Mock memories — matches MemoryListResponse
  await page.route('**/v1/memories*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        tenant_id: 'default',
        user_id: 'demo-user',
        memories: [
          { key: 'pref:dark-mode', content: 'User prefers dark mode interfaces' },
          { key: 'project:saas', content: 'Working on enterprise SaaS project with React + Ant Design' },
          { key: 'lang:typescript', content: 'Preferred language: TypeScript with strict mode' },
        ],
        count: 3,
      }),
    })
  })

  // Mock skills — matches SkillListResponse
  await page.route('**/v1/skills*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        tenant_id: 'default',
        skills: [
          { name: 'web-search', description: 'Search the web for real-time information', version: '1.2.0', source: 'builtin' },
          { name: 'code-exec', description: 'Execute code in sandboxed environment', version: '2.0.1', source: 'builtin' },
          { name: 'file-read', description: 'Read and analyze uploaded files', version: '1.0.0', source: 'plugin', user_modified: true },
          { name: 'image-gen', description: 'Generate images from text prompts', version: '0.9.0', source: 'plugin' },
        ],
        count: 4,
      }),
    })
  })

  // Mock usage — matches UsageResponse
  await page.route('**/v1/usage*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        tenant_id: 'default',
        period: '2025-05',
        input_tokens: 128450,
        output_tokens: 117230,
        total_tokens: 245680,
        estimated_cost_usd: 3.4200,
      }),
    })
  })

  // Mock audit logs — matches AuditLogListResponse
  await page.route('**/admin/v1/audit-logs*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        logs: [
          { id: 1, tenant_id: 'default', user_id: 'admin', action: 'api_key.created', detail: 'Created production key', request_id: 'req-001', status_code: 201, created_at: '2025-05-10T14:00:00Z' },
          { id: 2, tenant_id: 'tenant-1', user_id: 'admin', action: 'tenant.updated', detail: 'Updated tenant name', request_id: 'req-002', status_code: 200, created_at: '2025-05-10T12:30:00Z' },
          { id: 3, tenant_id: 'default', user_id: 'demo-user', action: 'user.login', detail: null, request_id: 'req-003', status_code: 200, created_at: '2025-05-10T10:00:00Z' },
        ],
        total: 3,
      }),
    })
  })

  // Mock pricing rules — matches PricingRuleListResponse
  await page.route('**/admin/v1/pricing-rules*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        rules: [
          { model_key: 'gpt-4o', input_per_1k: 0.005, output_per_1k: 0.015, cache_read_per_1k: 0.0025, updated_at: '2025-05-01T00:00:00Z' },
          { model_key: 'claude-3.5-sonnet', input_per_1k: 0.003, output_per_1k: 0.015, cache_read_per_1k: 0.0015, updated_at: '2025-04-20T00:00:00Z' },
        ],
      }),
    })
  })

  // Mock sandbox policy — matches SandboxPolicy
  await page.route('**/admin/v1/tenants/*/sandbox-policy*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        tenant_id: 'default',
        policy: JSON.stringify({ enabled: true, max_timeout_seconds: 30, allowed_tools: ['web-search', 'code-exec'], allow_docker: false, restrict_network: true, max_stdout_kb: 64 }, null, 2),
        updated_at: '2025-05-05T00:00:00Z',
      }),
    })
  })

  // Mock bootstrap status — matches BootstrapStatusResponse
  await page.route('**/admin/v1/bootstrap*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ bootstrap_required: false }),
    })
  })

  // Mock API keys — matches ApiKeyListResponse
  await page.route('**/admin/v1/tenants/*/api-keys*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        api_keys: [
          { id: 'key-1', name: 'Production Key', prefix: 'hx-prod', tenant_id: 'default', roles: ['admin'], scopes: ['all'], expires_at: null, revoked_at: null, created_at: '2025-03-01T00:00:00Z' },
          { id: 'key-2', name: 'Development Key', prefix: 'hx-dev', tenant_id: 'default', roles: ['user'], scopes: ['chat', 'read'], expires_at: '2025-12-31T00:00:00Z', revoked_at: null, created_at: '2025-04-15T00:00:00Z' },
        ],
        total: 2,
      }),
    })
  })

  if (type === 'user') {
    await page.goto('/')
    await page.locator('input[placeholder="API Key"]').fill('hx-test-key')
    await page.locator('input[placeholder="User ID"]').fill('demo-user')
    await page.locator('button:has-text("Connect")').click()
    await page.waitForTimeout(500)
  } else {
    await page.goto('/admin.html')
    await page.locator('input[placeholder="Admin API Key"]').fill('hx-admin-key')
    await page.locator('button:has-text("Sign In")').click()
    await page.waitForTimeout(500)
  }
}

test.describe('Authenticated User Portal (dark mode)', () => {
  test('Chat page with sidebar', async ({ page }) => {
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/chat')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-user-chat.png', fullPage: true })
  })

  test('Memories page with data', async ({ page }) => {
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/memories')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-user-memories.png', fullPage: true })
  })

  test('Skills page with data', async ({ page }) => {
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/skills')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-user-skills.png', fullPage: true })
  })

  test('Usage page with data', async ({ page }) => {
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/usage')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-user-usage.png', fullPage: true })
  })

  test('Settings page', async ({ page }) => {
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/settings')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-user-settings.png', fullPage: true })
  })

  test('Notifications page', async ({ page }) => {
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/notifications')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-user-notifications.png', fullPage: true })
  })
})

test.describe('Authenticated Admin Console (dark mode)', () => {
  test('Dashboard', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/dashboard')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-admin-dashboard.png', fullPage: true })
  })

  test('Tenants', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/tenants')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-admin-tenants.png', fullPage: true })
  })

  test('Users', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/users')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-admin-users.png', fullPage: true })
  })

  test('API Keys', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/keys')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-admin-apikeys.png', fullPage: true })
  })

  test('Audit Logs', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/audit')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-admin-audit.png', fullPage: true })
  })

  test('Pricing', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/pricing')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-admin-pricing.png', fullPage: true })
  })

  test('Sandbox', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/sandbox')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-admin-sandbox.png', fullPage: true })
  })

  test('System Settings', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/settings')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/auth-admin-settings.png', fullPage: true })
  })
})

test.describe('Light Mode Tests', () => {
  test('User Chat - light mode', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('hx-theme', 'light')
    })
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/chat')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/light-user-chat.png', fullPage: true })
  })

  test('User Memories - light mode', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('hx-theme', 'light')
    })
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/memories')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/light-user-memories.png', fullPage: true })
  })

  test('Admin Dashboard - light mode', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('hx-theme', 'light')
    })
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/dashboard')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/light-admin-dashboard.png', fullPage: true })
  })

  test('Admin Tenants - light mode', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('hx-theme', 'light')
    })
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/tenants')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/light-admin-tenants.png', fullPage: true })
  })
})

test.describe('Mobile Authenticated', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test('User Chat - mobile', async ({ page }) => {
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/chat')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/mobile-auth-user-chat.png', fullPage: true })
  })

  test('User Memories - mobile', async ({ page }) => {
    await mockAuthAndLogin(page, 'user')
    await page.goto('/#/memories')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/mobile-auth-user-memories.png', fullPage: true })
  })

  test('Admin Dashboard - mobile', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/dashboard')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/mobile-auth-admin-dashboard.png', fullPage: true })
  })

  test('Admin API Keys - mobile', async ({ page }) => {
    await mockAuthAndLogin(page, 'admin')
    await page.goto('/admin.html#/keys')
    await page.waitForTimeout(1500)
    await page.screenshot({ path: 'e2e/screenshots/mobile-auth-admin-apikeys.png', fullPage: true })
  })
})
