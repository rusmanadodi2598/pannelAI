// Vitest configuration.
//
// The SvelteKit plugin is loaded so `$lib` and the generated tsconfig resolve exactly as they do in
// the build, which keeps tests from passing against paths the app cannot use. `svelteTesting()` is
// required for the component tests: without it Svelte resolves to its server build and `mount()` is
// unavailable, which fails every test that renders a component.

import { sveltekit } from '@sveltejs/kit/vite';
import { svelteTesting } from '@testing-library/svelte/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [sveltekit(), svelteTesting()],
	test: {
		environment: 'jsdom',
		include: ['tests/**/*.test.ts'],
		restoreMocks: true
	}
});
