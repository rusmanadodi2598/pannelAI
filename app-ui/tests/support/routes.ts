// Route discovery for the panel's tests.
//
// The route table is derived from the filesystem rather than restated in a test, so adding a route or a
// navigation item re-checks itself with no change to the assertion. A `[bracket]` segment is kept literal,
// so a dynamic route never matches a concrete `href` by accident.

import { readdirSync, statSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';

const ROUTES_DIR = resolve(process.cwd(), 'src/routes');

export function discoverRoutes(dir: string = ROUTES_DIR): Set<string> {
	const routes = new Set<string>();

	for (const entry of readdirSync(dir)) {
		const full = join(dir, entry);

		if (statSync(full).isDirectory()) {
			for (const nested of discoverRoutes(full)) routes.add(nested);
			continue;
		}

		// A directory with a `+page.svelte` or `+page.ts` is a route; anything else (a layout, an error
		// page) is not, because it has no address of its own.
		if (entry !== '+page.svelte' && entry !== '+page.ts') continue;

		const segments = relative(ROUTES_DIR, dir)
			.split('/')
			.filter((segment) => segment.length > 0);
		routes.add(`/${segments.join('/')}`.replace(/\/$/, '') || '/');
	}

	return routes;
}

export const ROUTES = discoverRoutes();
