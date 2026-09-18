// Post-login redirect tests.
//
// The destination travels in a query string, so it is attacker-influenced input that decides where the
// browser goes next. SPEC-UI §8.1 asks for the requested route to be restored, and this suite is what
// keeps that from becoming an open redirect: it walks the shapes that must be refused as well as the
// paths that must be restored.

import { describe, expect, it } from 'vitest';
import { loginRedirectTarget, loginUrl, safeRedirectTarget } from '$lib/utils/redirect';

describe('safeRedirectTarget', () => {
	const CASES: { value: string | null | undefined; expected: string | null; why: string }[] = [
		{ value: '/endpoint-keys', expected: '/endpoint-keys', why: 'a typical protected route' },
		{ value: '/', expected: '/', why: 'the panel root' },
		{
			value: '/usage?page=2&per_page=50',
			expected: '/usage?page=2&per_page=50',
			why: 'a path with a query'
		},
		{ value: '/a/b/c/d/e/f', expected: '/a/b/c/d/e/f', why: 'a deep path' },
		{ value: '/login', expected: '/login', why: 'origin safety is not the login exclusion' },
		{ value: '  /usage  ', expected: '/usage', why: 'surrounding whitespace is trimmed' },
		{ value: '//evil.example/path', expected: null, why: 'protocol-relative is another origin' },
		{
			value: '/\\evil.example',
			expected: null,
			why: 'the backslash form some browsers read as //'
		},
		{ value: 'https://evil.example', expected: null, why: 'an absolute URL is not a path' },
		{ value: 'javascript:alert(1)', expected: null, why: 'a script URL' },
		{ value: 'data:text/html,<b>x</b>', expected: null, why: 'a data URL' },
		{ value: 'relative/path', expected: null, why: 'a relative path is not absolute' },
		{ value: '', expected: null, why: 'an empty value' },
		{ value: '   ', expected: null, why: 'a whitespace-only value' },
		{ value: null, expected: null, why: 'no value at all' },
		{ value: undefined, expected: null, why: 'an absent query parameter' },
		{ value: '/bad\npath', expected: null, why: 'a newline smuggled into the path' },
		{ value: '/bad\u0000path', expected: null, why: 'a null byte smuggled into the path' }
	];

	for (const testCase of CASES) {
		it(`returns ${testCase.expected ?? 'nothing'} for ${testCase.why}`, () => {
			expect(safeRedirectTarget(testCase.value)).toBe(testCase.expected);
		});
	}
});

describe('loginRedirectTarget', () => {
	const FALLBACK = '/endpoint-keys';

	const CASES: { requested: string | null | undefined; expected: string; why: string }[] = [
		{ requested: '/usage', expected: '/usage', why: 'a safe requested route is restored' },
		{ requested: '/usage?page=3', expected: '/usage?page=3', why: 'the query is restored too' },
		{ requested: '/', expected: '/', why: 'the root is a real destination' },
		{ requested: null, expected: FALLBACK, why: 'nothing requested falls back' },
		{ requested: '//evil.example', expected: FALLBACK, why: 'an unsafe value falls back' },
		{ requested: 'https://evil.example', expected: FALLBACK, why: 'an absolute URL falls back' },
		{ requested: '/login', expected: FALLBACK, why: 'the login screen would re-run the redirect' },
		{ requested: '/login?redirectTo=/x', expected: FALLBACK, why: 'the login screen with a query' },
		{ requested: '/login/again', expected: FALLBACK, why: 'a path under the login screen' }
	];

	for (const testCase of CASES) {
		it(`lands on ${testCase.expected} when ${testCase.why}`, () => {
			expect(loginRedirectTarget(testCase.requested, FALLBACK)).toBe(testCase.expected);
		});
	}
});

describe('loginUrl', () => {
	const CASES: { from: string; expected: string; why: string }[] = [
		{
			from: '/endpoint-keys',
			expected: '/login?redirectTo=%2Fendpoint-keys',
			why: 'a plain path is encoded'
		},
		{
			from: '/usage?page=2',
			expected: '/login?redirectTo=%2Fusage%3Fpage%3D2',
			why: 'the query is part of the destination'
		},
		{
			from: '/a b',
			expected: '/login?redirectTo=%2Fa%20b',
			why: 'a space is encoded rather than left raw'
		}
	];

	for (const testCase of CASES) {
		it(`builds a login URL when ${testCase.why}`, () => {
			expect(loginUrl(testCase.from)).toBe(testCase.expected);
		});
	}

	it('round-trips through the redirect target', () => {
		// The URL builder and the target reader are two halves of one contract, so this checks they agree
		// rather than only that each matches a literal.
		const url = loginUrl('/usage?page=7');
		const query = url.slice(url.indexOf('?') + 1);

		expect(
			loginRedirectTarget(new URLSearchParams(query).get('redirectTo'), '/endpoint-keys')
		).toBe('/usage?page=7');
	});
});
