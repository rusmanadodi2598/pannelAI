// Vite configuration for the panel.
//
// There is no dev-only CORS proxy here on purpose. The panel forwards /api/v1 from its own server
// (src/hooks.server.ts), so development and production share one code path and the session cookie
// stays first-party. See docs/SPEC-UI/001-SPEC-UI.md §3.1 and §10.1.3.

import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		host: '127.0.0.1',
		port: 5173
	}
});
