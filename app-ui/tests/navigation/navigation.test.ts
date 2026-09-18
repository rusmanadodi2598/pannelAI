// Navigation contract tests.
//
// The sidebar is generated from `src/lib/navigation.ts`, so the thing worth testing is that data: which
// groups exist, which items are real links, and whether a link can be written for a screen that does not
// exist. R-24 forbids a navigation item pointing at a missing route, and this is where that rule is
// enforced mechanically rather than by review.
//
// The route table comes from tests/support/routes.ts, so adding a route or an item re-checks itself with no
// change to this file. Contrast with the icon map test, which reads the other side of the same data.

import { describe, expect, it } from 'vitest';
import { NAV_GROUPS, type NavNode } from '$lib/navigation';
import { ROUTES } from '../support/routes';

// Flattened view of the navigation tree, with the group each node came from. Children are included because
// a Media kind is a navigation row too, and it satisfies the same rules.
function flatten(): { group: string; node: NavNode; depth: number }[] {
	const rows: { group: string; node: NavNode; depth: number }[] = [];

	function walk(group: string, nodes: NavNode[], depth: number): void {
		for (const node of nodes) {
			rows.push({ group, node, depth });
			if (node.children) walk(group, node.children, depth + 1);
		}
	}

	for (const group of NAV_GROUPS) walk(group.key, group.items, 0);

	return rows;
}

const ALL = flatten();
const NODES = ALL.map((row) => row.node);
const KEYS = NODES.map((node) => node.key);

// The screens the owner listed for the sidebar (owner direction, 2026-09-17). This is the requirement, so
// it is stated as data; the navigation tree is what has to satisfy it.
const OWNER_ITEMS: { label: string; key: string }[] = [
	{ key: 'endpoint-keys', label: 'Endpoint & Key' },
	{ key: 'providers', label: 'Provider' },
	{ key: 'combos', label: 'Combo & Vision Adapter' },
	{ key: 'usage', label: 'Usage' },
	{ key: 'quota', label: 'Quota Tracker' },
	{ key: 'token-saver', label: 'Token Saver' },
	{ key: 'skills', label: 'Skill' },
	{ key: 'media-embedding', label: 'Embedding' },
	{ key: 'media-image', label: 'Image' },
	{ key: 'media-video', label: 'Video' },
	{ key: 'media-tts', label: 'TTS' },
	{ key: 'media-stt', label: 'STT' },
	{ key: 'media-search', label: 'Web Search' },
	{ key: 'playground', label: 'Playground Chat' },
	{ key: 'proxies', label: 'Proxy Pools' },
	{ key: 'api-docs', label: 'API Docs' },
	{ key: 'changelog', label: 'Changelog' },
	{ key: 'console-log', label: 'Console Log' },
	{ key: 'settings', label: 'Setting' }
];

describe('navigation shape', () => {
	it('groups the sidebar instead of listing every screen flat', () => {
		// A flat list of nineteen rows is a list, not a structure. DESIGN.md §11 records the reason for
		// grouping: the group is the operator's question, not the screen's type.
		expect(NAV_GROUPS.length).toBeGreaterThanOrEqual(3);
		expect(NAV_GROUPS.length).toBeLessThanOrEqual(6);
	});

	for (const group of NAV_GROUPS) {
		it(`gives the ${group.key} group a label, a reason, and at least one item`, () => {
			expect(group.label.trim().length, `${group.key} needs a label`).toBeGreaterThan(0);
			expect(group.reason.length, `${group.key} needs a real reason`).toBeGreaterThanOrEqual(20);
			expect(group.items.length, `${group.key} is empty`).toBeGreaterThan(0);
		});
	}

	it('has unique group keys', () => {
		const keys = NAV_GROUPS.map((group) => group.key);
		expect(new Set(keys).size, `duplicate group key in ${keys.join(', ')}`).toBe(keys.length);
	});

	it('has unique item keys across the whole tree', () => {
		expect(new Set(KEYS).size, `duplicate item key in ${KEYS.join(', ')}`).toBe(KEYS.length);
	});

	it('has no duplicate label inside one group', () => {
		for (const group of NAV_GROUPS) {
			const labels = group.items.map((item) => item.label);
			expect(new Set(labels).size, `${group.key} repeats a label in ${labels.join(', ')}`).toBe(
				labels.length
			);
		}
	});
});

describe('navigation items', () => {
	for (const { node } of ALL) {
		it(`gives ${node.key} a clean label`, () => {
			expect(node.label, `${node.key} label`).toBe(node.label.trim());
			expect(node.label.length, `${node.key} label is empty`).toBeGreaterThan(0);
			expect(node.label, `${node.key} carries an em dash (R-02)`).not.toMatch(/[\u2014\u2013]/);
			expect(node.label, `${node.key} carries an emoji`).not.toMatch(/\p{Extended_Pictographic}/u);
		});

		it(`points ${node.key} at a route that exists, or at nothing at all`, () => {
			// A node without an href is the only correct state for a screen that is not built: the renderer
			// cannot produce a link for it, so R-24 cannot be violated by accident.
			if (node.href === undefined) return;

			expect(
				ROUTES.has(node.href),
				`${node.key} links to ${node.href}, which no route file provides`
			).toBe(true);
		});
	}

	it('points every link at a static screen, because a nav row cannot choose a parameter', () => {
		// The provider detail route is `/providers/[provider_id]`, and no sidebar row may link to it: the
		// renderer would have to invent a provider id, and `resolve` cannot build the link without one. The
		// type excludes it, and this asserts the same rule over the data, so a hand-edited href fails here
		// rather than at build time only.
		for (const { node } of ALL) {
			if (node.href === undefined) continue;
			expect(node.href, `${node.key} links to a parameterised route`).not.toContain('[');
		}
	});

	it('covers every screen the owner listed', () => {
		for (const required of OWNER_ITEMS) {
			const node = NODES.find((candidate) => candidate.key === required.key);
			expect(node, `${required.key} is missing from the sidebar`).toBeDefined();
			expect(node?.label, `${required.key} label`).toBe(required.label);
		}
	});

	it('offers no item the owner did not list', () => {
		// The reverse direction, so an invented screen cannot appear in the sidebar. A node that exists only
		// to hold children (the Media Provider container) is exempt.
		const declared = new Set(OWNER_ITEMS.map((item) => item.key));
		const containers = new Set(['media-providers', 'media']);

		for (const key of KEYS) {
			if (containers.has(key)) continue;
			expect(declared.has(key), `${key} is not in the owner list`).toBe(true);
		}
	});

	it('puts the media kinds under one expandable container', () => {
		const media = NODES.find((node) => node.children !== undefined);
		expect(media, 'no navigation node has children').toBeDefined();
		expect(media?.key).toBe('media-providers');
		expect(media?.href, 'the media container is not a route').toBeUndefined();

		const kinds = media?.children?.map((child) => child.key).sort() ?? [];
		expect(kinds).toEqual([
			'media-embedding',
			'media-image',
			'media-search',
			'media-stt',
			'media-tts',
			'media-video'
		]);
	});

	it('marks a planned screen as planned rather than hiding it', () => {
		// The owner asked for the full structure up front, so every unbuilt screen is visible and labelled.
		// A hidden item would misrepresent the panel's scope. A container is not a screen: it carries
		// children and its own label, so it is exempt.
		for (const { node } of ALL) {
			if (node.href !== undefined || node.children !== undefined) continue;
			expect(node.planned, `${node.key} has no route and is not marked planned`).toBe(true);
		}
	});

	it('gives every child of a container a planned state of its own', () => {
		// A container is exempt from the chip, so its children have to carry the information instead.
		const container = NODES.find((node) => node.children !== undefined);
		for (const child of container?.children ?? []) {
			expect(child.href, `${child.key} is a leaf and must not carry an href yet`).toBeUndefined();
			expect(child.planned, `${child.key} needs its own planned state`).toBe(true);
		}
	});
});
