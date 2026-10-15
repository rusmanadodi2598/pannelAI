const { test, expect } = require('@playwright/test');

test('signed-out route redirects to login and preserves the requested path', async ({ page }) => {
  await page.goto('/endpoint-keys');

  await expect(page).toHaveURL(/\/login\?redirectTo=/);
  await expect(page.getByRole('heading', { name: 'Sign in to pannelAI' })).toBeVisible();
  expect(new URL(page.url()).searchParams.get('redirectTo')).toBe('/endpoint-keys');
});

test('invalid password shows the expected inline error', async ({ page }) => {
  await page.goto('/login');
  await page.getByLabel('Panel password').fill('incorrect-password');
  await page.getByRole('button', { name: 'Sign in' }).click();

  await expect(page.getByRole('alert')).toContainText('Wrong password. Try again.');
  await expect(page.getByLabel('Panel password')).toHaveValue('');
});

test('successful login returns to the requested protected route', async ({ page }) => {
  await page.goto('/login?redirectTo=%2Fendpoint-keys');
  await page.getByLabel('Panel password').fill('browser-e2e-password');
  await page.getByRole('button', { name: 'Sign in' }).click();

  await expect(page).toHaveURL(/\/endpoint-keys(?:\?.*)?$/);
  await expect(page.getByRole('heading', { name: 'Endpoint & Key' })).toBeVisible();
});

test('login screen fits a narrow mobile viewport without horizontal overflow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/login');
  await expect(page.getByRole('heading', { name: 'Sign in to pannelAI' })).toBeVisible();

  const dimensions = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    content: document.documentElement.scrollWidth
  }));
  expect(dimensions.content).toBeLessThanOrEqual(dimensions.viewport);
});
