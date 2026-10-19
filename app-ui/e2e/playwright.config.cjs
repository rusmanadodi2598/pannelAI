const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
	testDir: __dirname,
	testMatch: '**/*.spec.cjs',
	fullyParallel: true,
	forbidOnly: Boolean(process.env.CI),
	retries: process.env.CI ? 1 : 0,
	workers: process.env.CI ? 2 : undefined,
	reporter: process.env.CI ? 'line' : 'list',
	use: {
		baseURL: process.env.PANEL_BASE_URL || 'http://127.0.0.1:5173',
		browserName: 'chromium',
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure'
	}
});
