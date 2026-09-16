// Navigation icon map tests.
//
// R-04 requires an icon to be relevant and its purpose writable in one line, so the map is checked as
// data: a key without an entry, an entry without a reason, or an entry for something that is not a
// screen fails here. The key list is a table, so a screen added to the sidebar is a row rather than
// another copy of the same test.

import { describe, expect, it } from 'vitest';
import { NAV_ICONS, navIcon } from '$lib/icons';

const SCREEN_KEYS = [
	'endpoint-keys',
	'providers',
	'combos',
	'usage',
	'quota',
	'token-saver',
	'media-embedding',
	'media-image',
	'media-video',
	'media-tts',
	'media-stt',
	'media-search',
	'proxies',
	'skills',
	'logs',
	'api-docs',
	'settings'
];

const KEY_CASES = [
	...SCREEN_KEYS.map((key) => ({ key, defined: true })),
	{ key: 'not-a-screen', defined: false },
	{ key: 'ENDPOINT-KEYS', defined: false },
	{ key: 'media-audio', defined: false },
	{ key: 'endpoint-keys ', defined: false },
	{ key: '', defined: false }
];

describe('navigation icon map', () => {
	for (const testCase of KEY_CASES) {
		it(`${testCase.defined ? 'resolves' : 'rejects'} ${JSON.stringify(testCase.key)}`, () => {
			const entry = navIcon(testCase.key);

			expect(entry === undefined, `${testCase.key} resolved to ${entry?.reason ?? 'nothing'}`).toBe(
				!testCase.defined
			);
		});
	}

	it('has no entries beyond the navigation keys', () => {
		for (const key of Object.keys(NAV_ICONS)) {
			expect(SCREEN_KEYS, `${key} is not a navigation key`).toContain(key);
		}
	});

	it('gives every icon a reason of at least 20 characters', () => {
		for (const [key, entry] of Object.entries(NAV_ICONS)) {
			expect(entry.reason.length, `${key} needs a real reason`).toBeGreaterThanOrEqual(20);
		}
	});
});
