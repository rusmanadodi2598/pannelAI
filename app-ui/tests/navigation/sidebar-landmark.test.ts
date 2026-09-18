// Sidebar landmark test.
//
// The navigation is a `nav` landmark with an accessible name, so a screen-reader user can skip the shell
// and land on the panel's links. This renders the real PanelSidebar instead of asserting on the source,
// because the landmark has to survive both render paths (the desktop container and the mobile Sheet) and
// a string match would not notice if it moved outside the children they share.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SidebarHarness from '../support/SidebarHarness.svelte';

vi.mock('$app/state', () => ({
	page: { url: { pathname: '/endpoint-keys' } }
}));

vi.mock('$app/paths', () => ({
	resolve: (path: string) => path
}));

// The sidebar's mobile check reads a media query, which jsdom does not implement. Reporting "not mobile"
// keeps the desktop branch under test; the mobile branch draws the same children inside a Sheet.
function stubMatchMedia(): void {
	window.matchMedia = ((query: string) => ({
		matches: false,
		media: query,
		onchange: null,
		addEventListener: () => {},
		removeEventListener: () => {},
		addListener: () => {},
		removeListener: () => {},
		dispatchEvent: () => false
	})) as unknown as typeof window.matchMedia;
}

// The top-level owner items. The Media kinds sit in a disclosure that is closed at rest, so they are not
// asserted here; navigation.test.ts owns the full tree.
const TOP_LEVEL_LABELS = [
	'Endpoint & Key',
	'Provider',
	'Combo & Vision Adapter',
	'Usage',
	'Quota Tracker',
	'Token Saver',
	'Skill',
	'Media Provider',
	'Playground Chat',
	'Proxy Pools',
	'API Docs',
	'Changelog',
	'Console Log',
	'Setting'
];

describe('sidebar navigation landmark', () => {
	beforeEach(stubMatchMedia);
	afterEach(cleanup);

	it('exposes exactly one named navigation landmark', () => {
		render(SidebarHarness);

		expect(screen.getAllByRole('navigation', { name: 'Panel navigation' })).toHaveLength(1);
	});

	it('puts every owner item inside the landmark', () => {
		render(SidebarHarness);

		const nav = screen.getByRole('navigation', { name: 'Panel navigation' });

		for (const label of TOP_LEVEL_LABELS) {
			expect(nav.textContent, `${label} is outside the navigation landmark`).toContain(label);
		}
	});
});
