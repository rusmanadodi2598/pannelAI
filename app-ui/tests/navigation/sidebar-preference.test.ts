// Sidebar preference tests.
//
// DESIGN.md §8 gives the three breakpoints different behaviour: a drawer under 768px, an icon rail on a
// tablet, and a full sidebar on a desktop. That means the initial open state depends on the viewport,
// not only on what the operator chose last, and this is the logic worth testing because it is the part
// that silently gets it wrong on a tablet.

import { describe, expect, it } from 'vitest';
import { resolveSidebarOpen, SIDEBAR_PREFERENCE_MAX_AGE, sidebarCookie } from '$lib/stores/sidebar';

describe('resolveSidebarOpen', () => {
	const CASES: { stored: string | null; desktop: boolean; expected: boolean; why: string }[] = [
		{
			stored: 'true',
			desktop: true,
			expected: true,
			why: 'desktop with an explicit expanded choice'
		},
		{ stored: 'true', desktop: false, expected: true, why: 'tablet, operator expanded it before' },
		{
			stored: 'false',
			desktop: true,
			expected: false,
			why: 'desktop with an explicit collapsed choice'
		},
		{
			stored: 'false',
			desktop: false,
			expected: false,
			why: 'tablet, operator collapsed it before'
		},
		{
			stored: null,
			desktop: true,
			expected: true,
			why: 'first visit on a desktop opens the sidebar'
		},
		{
			stored: null,
			desktop: false,
			expected: false,
			why: 'first visit on a tablet shows the rail'
		},
		{ stored: '', desktop: true, expected: true, why: 'an empty cookie is not a choice' },
		{ stored: '1', desktop: false, expected: false, why: 'a non-boolean cookie is not a choice' },
		{
			stored: 'TRUE',
			desktop: false,
			expected: false,
			why: 'the cookie is written lowercase only'
		},
		{ stored: 'yes', desktop: true, expected: true, why: 'garbage falls back to the viewport' },
		{
			stored: 'false; other=1',
			desktop: true,
			expected: false,
			why: 'a trailing attribute is trimmed'
		}
	];

	for (const testCase of CASES) {
		it(`${testCase.why}`, () => {
			expect(resolveSidebarOpen(testCase.stored, testCase.desktop)).toBe(testCase.expected);
		});
	}

	it('treats a value with surrounding whitespace as the value it is', () => {
		// A cookie value is not trimmed by the browser, so a stray space must not flip the meaning.
		expect(resolveSidebarOpen(' true', false)).toBe(true);
		expect(resolveSidebarOpen('false ', true)).toBe(false);
	});
});

describe('sidebarCookie', () => {
	it('writes a path-wide, long-lived preference', () => {
		const cookie = sidebarCookie(false);

		expect(cookie).toContain('sidebar_state=false');
		expect(cookie).toContain('path=/');
		expect(cookie).toContain(`max-age=${SIDEBAR_PREFERENCE_MAX_AGE}`);
	});

	it('writes the expanded state as true', () => {
		expect(sidebarCookie(true)).toContain('sidebar_state=true');
	});
});
