// Navigation icon map tests.
//
// R-04 and R-31 require an icon to be relevant and its purpose writable in one line, so the map is
// checked as data: an entry without a reason, or a reason too short to mean anything, fails here.

import { describe, expect, it } from 'vitest';
import { NAV_ICONS, navIcon } from '$lib/icons';

const KEYS = [
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

describe('navigation icon map', () => {
	it('covers every navigation key', () => {
		for (const key of KEYS) {
			expect(NAV_ICONS[key], `missing icon entry for ${key}`).toBeDefined();
		}
	});

	it('has no entries beyond the navigation keys', () => {
		expect(Object.keys(NAV_ICONS).sort()).toEqual([...KEYS].sort());
	});

	it('gives every icon a reason of at least 20 characters', () => {
		for (const [key, entry] of Object.entries(NAV_ICONS)) {
			expect(entry.reason.length, `${key} needs a real reason`).toBeGreaterThanOrEqual(20);
		}
	});

	it('resolves a known key and returns undefined for an unknown one', () => {
		expect(navIcon('settings')).toBeDefined();
		expect(navIcon('not-a-screen')).toBeUndefined();
	});
});
