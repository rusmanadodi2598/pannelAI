const { test, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

// This is a route-wide visual smoke baseline, not a claim that API-backed data is populated.
// Every API request is deterministically failed except auth status, so the real UI must render
// each page's loading/error/empty handling without a live gateway or production credentials.
const routes = [
  { path: '/endpoint-keys', name: 'endpoint-keys' },
  { path: '/providers', name: 'providers' },
  { path: '/providers/e2e-provider', name: 'provider-detail' },
  { path: '/combos', name: 'combos' },
  { path: '/usage', name: 'usage' },
  { path: '/quota', name: 'quota' },
  { path: '/token-saver', name: 'token-saver' },
  { path: '/media-providers/embedding', name: 'media-embedding' },
  { path: '/media-providers/image', name: 'media-image' },
  { path: '/media-providers/video', name: 'media-video' },
  { path: '/media-providers/tts', name: 'media-tts' },
  { path: '/media-providers/stt', name: 'media-stt' },
  { path: '/media-providers/web', name: 'media-web' },
  { path: '/proxy-pools', name: 'proxy-pools' },
  { path: '/skills', name: 'skills' },
  { path: '/logs', name: 'logs' },
  { path: '/console-log', name: 'console-log' },
  { path: '/playground', name: 'playground' },
  { path: '/changelog', name: 'changelog' },
  { path: '/api-docs', name: 'api-docs' },
  { path: '/settings', name: 'settings' }
];

const viewports = [
  { name: 'desktop-1440', width: 1440, height: 1000 },
  { name: 'mobile-390', width: 390, height: 844 }
];
const themes = ['light', 'dark'];

test.describe('whole-dashboard visual route audit', () => {
  test.setTimeout(45_000);

  for (const route of routes) {
    for (const viewport of viewports) {
      for (const theme of themes) {
        test(`${route.name} / ${viewport.name} / ${theme}`, async ({ browser }, testInfo) => {
          const context = await browser.newContext({
            viewport: { width: viewport.width, height: viewport.height },
            colorScheme: theme
          });
          await context.addInitScript((name) => localStorage.setItem('pannelai.theme', name), theme);
          const page = await context.newPage();

          await page.route('**/api/v1/**', async (routeRequest) => {
            const url = new URL(routeRequest.request().url());
            if (url.pathname === '/api/v1/auth/status') {
              return routeRequest.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                  authenticated: true,
                  require_login: false,
                  password_configured: true
                })
              });
            }
            return routeRequest.fulfill({
              status: 503,
              contentType: 'application/json',
              body: JSON.stringify({
                error: {
                  code: 'VISUAL_AUDIT_FIXTURE',
                  message: 'Deterministic visual-audit fixture: live API is intentionally unavailable.'
                }
              })
            });
          });

          try {
            await page.goto(route.path, { waitUntil: 'domcontentloaded' });
            await expect(page.locator('main')).toBeVisible({ timeout: 10_000 });
            await page.waitForTimeout(250);

            const dimensions = await page.evaluate(() => ({
              viewport: document.documentElement.clientWidth,
              document: document.documentElement.scrollWidth,
              main: Boolean(document.querySelector('main'))
            }));
            expect(dimensions.main, 'main content landmark should exist').toBe(true);
            expect(
              dimensions.document,
              `horizontal overflow at ${route.path}: viewport=${dimensions.viewport}, document=${dimensions.document}`
            ).toBeLessThanOrEqual(dimensions.viewport + 1);

            const outputDir = testInfo.outputPath('visual-audit');
            fs.mkdirSync(outputDir, { recursive: true });
            const filename = `${route.name}--${viewport.name}--${theme}.png`;
            await page.screenshot({
              path: path.join(outputDir, filename),
              animations: 'disabled',
              caret: 'hide'
            });
          } finally {
            await context.close();
          }
        });
      }
    }
  }
});
