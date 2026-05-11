const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: '.',
  testMatch: /docs-site\.spec\.js/,
  timeout: 60000,
  retries: 1,
  workers: 3,
  reporter: [['list'], ['html', { outputFolder: '../playwright-report', open: 'never' }]],
  use: {
    headless: true,
    viewport: { width: 1280, height: 900 },
    ignoreHTTPSErrors: false,
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
