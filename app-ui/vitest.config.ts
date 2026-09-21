// Vitest configuration.
//
// The SvelteKit plugin is loaded so `$lib` and the generated tsconfig resolve exactly as they do in
// the build, which keeps tests from passing against paths the app cannot use.

import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [sveltekit()],
	test: {
		environment: 'jsdom',
		include: ['tests/**/*.test.ts'],
		restoreMocks: true
	}
});
