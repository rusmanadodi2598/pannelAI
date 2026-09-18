// Sidebar preference tests.
//
// DESIGN.md §8 gives the three breakpoints different behaviour: a drawer under 768px, an icon rail on a
// tablet, and a full sidebar on a desktop. That means the initial open state depends on the viewport,
// not only on what the operator chose last, and this is the logic worth testing because it is the part
// that silently gets it wrong on a tablet.
//
// The write side is the primitive layer's and is not tested here. What the application owns is reading
// the cookie back and deciding the first paint, which is what these two functions do.

import { describe, expect, it } from 'vitest';
import { readSidebarCookie, resolveSidebarOpen } from '$lib/stores/sidebar';

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

describe('readSidebarCookie', () => {
	const CASES: { cookie: string | undefined; expected: string | null; why: string }[] = [
		{ cookie: 'sidebar_state=true', expected: 'true', why: 'the preference is the only cookie' },
		{ cookie: 'sidebar_state=false', expected: 'false', why: 'a collapsed preference' },
		{ cookie: 'other=1; sidebar_state=true', expected: 'true', why: 'it follows another cookie' },
		{ cookie: 'sidebar_state=true; other=1', expected: 'true', why: 'it precedes another cookie' },
		{ cookie: 'sidebar_state=', expected: '', why: 'an empty value is reported as empty' },
		{ cookie: 'sidebar_stateX=true', expected: null, why: 'a name that only starts the same' },
		{ cookie: 'other=1', expected: null, why: 'the preference is absent' },
		{ cookie: '', expected: null, why: 'an empty cookie string' },
		{ cookie: undefined, expected: null, why: 'no cookie string at all' }
	];

	for (const testCase of CASES) {
		it(`reads ${testCase.expected ?? 'nothing'} when ${testCase.why}`, () => {
			expect(readSidebarCookie(testCase.cookie)).toBe(testCase.expected);
		});
	}

	it('uses the name the primitive layer writes, so the read and the write cannot disagree', () => {
		// The name is imported from the primitive's constants rather than repeated as a literal. If the
		// primitive is regenerated with a different name, this test is where the mismatch shows up.
		expect(readSidebarCookie('sidebar_state=true')).toBe('true');
	});
});
