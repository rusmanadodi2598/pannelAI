// Navigation icon map tests.
//
// R-04 requires an icon to be relevant and its purpose writable in one line, so the map is checked as
// data rather than as markup. The two sides are cross-checked: a navigation row with no icon and an
// icon entry with no row both fail, which is what stops the map from quietly drifting away from the
// sidebar it describes.
//
// The key list comes from `src/lib/navigation.ts`, so adding a row re-checks itself with no change here.

import { describe, expect, it } from 'vitest';
import { NAV_ICONS, ROW_ACTION_ICONS, STATUS_ICONS, navIcon, rowActionIcon } from '$lib/icons';
import { allNodes } from '$lib/navigation';

const NAV_KEYS = allNodes().map((node) => node.key);

// Keys that must never resolve. A near miss is the interesting case: `media-audio` and `proxy` look
// plausible and would silently render nothing, so they are asserted as rejections rather than assumed.
const REJECTED_KEYS = [
	'not-a-screen',
	'ENDPOINT-KEYS',
	'endpoint-keys ',
	' endpoint-keys',
	'media-audio',
	'proxy',
	'changelogs',
	''
];

describe('navigation icon map', () => {
	it('resolves an icon for every navigation row', () => {
		for (const key of NAV_KEYS) {
			expect(navIcon(key), `${key} has no icon entry`).toBeDefined();
		}
	});

	for (const key of REJECTED_KEYS) {
		it(`rejects ${JSON.stringify(key)}`, () => {
			expect(navIcon(key)).toBeUndefined();
		});
	}

	it('has no entries beyond the navigation rows and the extra status icons', () => {
		const allowed = new Set([...NAV_KEYS, ...Object.keys(STATUS_ICONS)]);

		for (const key of Object.keys(NAV_ICONS)) {
			expect(allowed.has(key), `${key} is not a navigation row`).toBe(true);
		}
	});

	it('gives every icon a reason of at least 20 characters', () => {
		for (const [key, entry] of [...Object.entries(NAV_ICONS), ...Object.entries(STATUS_ICONS)]) {
			expect(entry.reason.length, `${key} needs a real reason`).toBeGreaterThanOrEqual(20);
			expect(entry.reason, `${key} reason must be one sentence`).toMatch(/\.$/);
		}
	});

	// R-04: the banned glyphs are the generic "AI product" vocabulary. Asserted by importing the
	// resolved component names so a rename inside the library cannot slip a sparkle back in.
	it('uses no sparkle, star, magic, lightning, diamond, robot, or orb glyph', () => {
		const banned = /sparkle|star|magic|zap|lightning|diamond|bot|robot|orb|cube/i;

		for (const [key, entry] of Object.entries(NAV_ICONS)) {
			// A Svelte 5 component is callable but not a class, so only `name` is reliable here.
			const name = entry.icon.name ?? '';
			expect(banned.test(name), `${key} resolves to ${name}, which R-04 rejects`).toBe(false);
		}
	});

	it('gives the media container and its kinds distinct icons for the kinds that differ', () => {
		// Five of the six kinds must not share a glyph with each other, or the sub-items become
		// indistinguishable at a glance.
		const kinds = NAV_KEYS.filter((key) => key.startsWith('media-') && key !== 'media-providers');
		const icons = kinds.map((key) => navIcon(key)?.icon);

		expect(new Set(icons).size, 'two media kinds share an icon').toBe(kinds.length);
	});
});

// The Endpoint & Key tables render these as icon-only buttons, so the map is the only place a glyph is
// named and the button's accessible name comes from the component. A missing entry would leave a button
// with no glyph; a banned one would put the generic "AI product" vocabulary into an action row.
describe('row action icon map', () => {
	const ACTIONS = Object.keys(ROW_ACTION_ICONS) as (keyof typeof ROW_ACTION_ICONS)[];

	it('resolves every action the tables ask for', () => {
		for (const action of ACTIONS) {
			const entry = rowActionIcon(action);
			expect(entry, `${action} has no icon entry`).toBeDefined();
			expect(entry.icon.name, `${action} resolves to a nameless component`).toBeTruthy();
		}
	});

	it('gives every action a reason of at least 20 characters', () => {
		for (const [key, entry] of Object.entries(ROW_ACTION_ICONS)) {
			expect(entry.reason.length, `${key} needs a real reason`).toBeGreaterThanOrEqual(20);
			expect(entry.reason, `${key} reason must be one sentence`).toMatch(/\.$/);
		}
	});

	it('uses no sparkle, star, magic, lightning, diamond, robot, or orb glyph', () => {
		const banned = /sparkle|star|magic|zap|lightning|diamond|bot|robot|orb|cube/i;

		for (const [key, entry] of Object.entries(ROW_ACTION_ICONS)) {
			const name = entry.icon.name ?? '';
			expect(banned.test(name), `${key} resolves to ${name}, which R-04 rejects`).toBe(false);
		}
	});
});
