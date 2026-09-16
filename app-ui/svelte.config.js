// SvelteKit configuration for the panel.
//
// Adapter choice: the official Node adapter, executed by the Bun runtime.
// The Bun-specific SvelteKit adapter was checked at scaffold time and is still on a 1.0.x release,
// while the runtime requirement (Bun, not Node) is satisfied by running this output with `bun`.
// docs/SPEC-UI/001-SPEC-UI.md §10.1.2 allows this fallback and requires the reason to be recorded here.

import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter({ out: 'build' })
	}
};

export default config;
