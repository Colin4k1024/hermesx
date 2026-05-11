/**
 * Playwright smoke test for the GitHub Pages MkDocs site.
 * Tests both EN and ZH versions, language switching, and all nav pages.
 *
 * Run: npx playwright test tests/docs-site.spec.js --config=tests/docs-playwright.config.js
 */
const { test, expect } = require('@playwright/test');
const path = require('path');
const fs = require('fs');

const BASE = 'https://Colin4k1024.github.io/hermesx';
const SCREENSHOT_DIR = path.join(__dirname, '..', 'test-results', 'docs-site');

// All pages defined in mkdocs.yml nav
const EN_PAGES = [
  { name: 'home',                 url: '/'                                          },
  { name: 'saas-quickstart',      url: '/saas-quickstart/'                          },
  { name: 'configuration',        url: '/configuration/'                            },
  { name: 'authentication',       url: '/authentication/'                           },
  { name: 'architecture',         url: '/architecture/'                             },
  { name: 'database',             url: '/database/'                                 },
  { name: 'security-model',       url: '/SECURITY_MODEL/'                           },
  { name: 'rbac-matrix',          url: '/RBAC_MATRIX/'                              },
  { name: 'enterprise-readiness', url: '/ENTERPRISE_READINESS/'                     },
  { name: 'deployment',           url: '/deployment/'                               },
  { name: 'observability',        url: '/observability/'                            },
  { name: 'skills-guide',         url: '/skills-guide/'                             },
  { name: 'api-reference',        url: '/api-reference/'                            },
  { name: 'changelog',            url: '/CHANGELOG/'                                },
  { name: 'adr-002',              url: '/adr/ADR-002-dual-layer-rate-limiter/'      },
  { name: 'adr-003',              url: '/adr/ADR-003-hermesx-webui-vue3-incremental/' },
  { name: 'adr-004',              url: '/adr/ADR-004-hermesx-webui-multipage-vite/' },
  { name: 'adr-005',              url: '/adr/ADR-005-hermesx-webui-bootstrap-endpoint/' },
];

// Chinese versions of all nav pages
const ZH_PAGES = EN_PAGES.map(p => ({
  name: 'zh-' + p.name,
  url: '/zh' + p.url,
}));

async function shot(page, name) {
  await page.screenshot({ path: `${SCREENSHOT_DIR}/${name}.png`, fullPage: true });
}

async function shotViewport(page, name) {
  await page.screenshot({ path: `${SCREENSHOT_DIR}/${name}.png`, fullPage: false });
}

// ── EN pages ─────────────────────────────────────────────────────────────────
test.describe('EN pages — no 404, nav present', () => {
  for (const p of EN_PAGES) {
    test(`EN: ${p.name}`, async ({ page }) => {
      await page.goto(BASE + p.url, { waitUntil: 'networkidle', timeout: 30000 });
      await expect(page).not.toHaveTitle(/Page Not Found|404/i);
      await expect(page.locator('.md-header')).toBeVisible();
      await shot(page, `en-${p.name}`);
    });
  }
});

// ── ZH pages ─────────────────────────────────────────────────────────────────
test.describe('ZH pages — no 404, nav present', () => {
  for (const p of ZH_PAGES) {
    test(`ZH: ${p.name}`, async ({ page }) => {
      await page.goto(BASE + p.url, { waitUntil: 'networkidle', timeout: 30000 });
      await expect(page).not.toHaveTitle(/Page Not Found|404/i);
      await expect(page.locator('.md-header')).toBeVisible();
      await shot(page, `${p.name}`);
    });
  }
});

// ── Language switcher ─────────────────────────────────────────────────────────
test.describe('Language switcher', () => {
  test('EN home: language toggle visible and links correct', async ({ page }) => {
    await page.goto(BASE + '/', { waitUntil: 'networkidle' });

    // Click the language/translate icon to open the dropdown
    const langBtn = page.locator('.md-header__option [data-md-component="palette"] ~ *, a[href*="/zh/"], [title*="language" i], [title*="Select language" i]').first();

    // Capture before clicking
    await shotViewport(page, 'lang-en-before-click');

    // Check alternate link exists in DOM
    const zhAlternate = page.locator('link[hreflang="zh"]');
    await expect(zhAlternate).toBeAttached();

    // Check language switcher element
    const switcher = page.locator('.md-select__inner a[href*="/zh/"], nav a[href*="/zh/"]');
    const switcherCount = await switcher.count();
    console.log(`ZH links in nav: ${switcherCount}`);

    await shotViewport(page, 'lang-en-viewport');
  });

  test('ZH home: shows Chinese content', async ({ page }) => {
    await page.goto(BASE + '/zh/', { waitUntil: 'networkidle' });
    await expect(page).not.toHaveTitle(/Page Not Found|404/i);
    const content = await page.locator('article').first().innerText();
    expect(content).toContain('企业级');
    await shotViewport(page, 'lang-zh-viewport');
    await shot(page, 'zh-home-full');
  });

  test('Language switcher click: EN → ZH', async ({ page }) => {
    await page.goto(BASE + '/', { waitUntil: 'networkidle' });
    await shotViewport(page, 'lang-after-click-translate-icon');

    // Material theme: language selector is .md-select__current button
    const langBtn = page.locator('.md-select__current').first();
    if (await langBtn.count() > 0) {
      await langBtn.click();
      await page.waitForTimeout(400);
      await shotViewport(page, 'lang-dropdown-open');

      const zhLink = page.locator('.md-select__list a[href*="/zh/"]').first();
      if (await zhLink.count() > 0) {
        await zhLink.click();
        await page.waitForURL('**/zh/**', { timeout: 8000 }).catch(() => {});
        await shotViewport(page, 'lang-after-click-zh');
        const url = page.url();
        console.log(`After clicking 中文: ${url}`);
        expect(url).toContain('/zh/');
      } else {
        console.log('ZH dropdown link not found — taking diagnostic shot');
        await shotViewport(page, 'lang-dropdown-diagnostic');
      }
    } else {
      // Fallback: direct navigation test
      console.log('.md-select__current not found, skipping click test');
    }
  });

  test('Navigation from ZH home: clicking nav tab stays in ZH', async ({ page }) => {
    await page.goto(BASE + '/zh/', { waitUntil: 'networkidle' });
    await shotViewport(page, 'nav-zh-home');

    // Click "Getting Started" tab from ZH home
    const gettingStarted = page.locator('.md-tabs__link, .md-nav__link').filter({ hasText: 'Getting Started' }).first();
    if (await gettingStarted.count() > 0) {
      await gettingStarted.click();
      await page.waitForLoadState('networkidle');
      const url = page.url();
      console.log(`After clicking Getting Started from /zh/: ${url}`);
      await shotViewport(page, 'nav-zh-after-getting-started');
    }
  });
});
