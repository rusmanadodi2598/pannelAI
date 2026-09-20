// Sidebar disclosure tests.
//
// A container starts closed and opens only when one of its children is the current route, which is what
// keeps a reload on a Media kind page showing the row the operator is standing on. The check reads the
// resolved path rather than a route pattern, so a parameterised child counts the same as a static one.
// That is the rule this file pins: before the media kinds linked anywhere the check read `child.href`,
// and it would have stopped matching without failing anything else.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import SidebarHarness from '../support/SidebarHarness.svelte';
import { forEachCase } from '../support/tables';

// Hoisted so the `vi.mock` factory below can close over it, which is what lets a test set the route
// before rendering. The component reads the path once on mount, so a plain object is enough.
const pageState = vi.hoisted(() => ({ url: { pathname: '/' } }));

vi.mock('$app/state', () => ({ page: pageState }));

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
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

function mediaTrigger(): HTMLElement {
	return screen.getByRole('button', { name: 'Media Provider' });
}

function renderAt(pathname: string): void {
	pageState.url = { pathname };
	render(SidebarHarness);
}

beforeEach(stubMatchMedia);
afterEach(cleanup);

describe('a container opens on the route it holds', () => {
	forEachCase(
		[
			{ name: 'opens on the first media kind', pathname: '/media-providers/embedding', open: true },
			{
				name: 'opens on a kind in the middle of the list',
				pathname: '/media-providers/tts',
				open: true
			},
			{
				name: 'opens on the kind whose panel name differs from the API name',
				pathname: '/media-providers/web',
				open: true
			},
			{ name: 'stays closed on a screen in another group', pathname: '/proxy-pools', open: false },
			{
				name: 'stays closed on the registry, which is a sibling of the container',
				pathname: '/providers',
				open: false
			},
			{
				name: 'stays closed on a media address that names no kind, because no row is that page',
				pathname: '/media-providers/chat',
				open: false
			},
			{ name: 'stays closed at the root', pathname: '/', open: false }
		],
		({ pathname, open }) => {
			renderAt(pathname);
			expect(mediaTrigger().getAttribute('aria-expanded')).toBe(String(open));
		}
	);

	it('gives every kind row the address of its own kind', () => {
		// The disclosure is open on this route, so the six rows are the ones the operator can reach. Each
		// href is asserted because a copy-paste that pointed two rows at one kind would render one screen
		// twice and hide another.
		renderAt('/media-providers/tts');

		const kinds: [string, string][] = [
			['Embedding', 'embedding'],
			['Image', 'image'],
			['Video', 'video'],
			['TTS', 'tts'],
			['STT', 'stt'],
			['Web Search', 'web']
		];

		for (const [label, slug] of kinds) {
			const link = screen.getByRole('link', { name: label });
			expect(link.getAttribute('href'), `${label} link`).toBe(`/media-providers/${slug}`);
		}
	});

	it('marks the row for the current kind as the current page, and no other', () => {
		// Position is carried by `aria-current` as well as the accent marker, because DESIGN.md §6 requires
		// the marker never to be the only signal.
		renderAt('/media-providers/stt');

		expect(screen.getByRole('link', { name: 'STT' }).getAttribute('aria-current')).toBe('page');
		expect(screen.getByRole('link', { name: 'TTS' }).getAttribute('aria-current')).toBeNull();
	});
});
