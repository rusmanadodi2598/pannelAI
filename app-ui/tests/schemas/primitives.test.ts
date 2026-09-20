// Tests for the field primitives in src/lib/schemas/primitives.ts.
//
// One table per primitive, with the case types docs/RULLES/TDD.md §2.5 asks for: a typical value, a
// boundary, an empty value, an invalid value, and an extreme one. Acceptance and rejection share the
// table so a primitive cannot drift toward being too permissive without a row failing.

import { describe, expect, it } from 'vitest';
import {
	absoluteUrl,
	gatewayKeyInput,
	label,
	noProxyList,
	perPage,
	proxyHost,
	proxyPort,
	searchText,
	secretValue
} from '$lib/schemas/primitives';

type ParseCase = { name: string; input: unknown; ok: boolean };
type MapCase<T> = { name: string; input: unknown; expected: T | null };

function forEachParse(
	cases: ParseCase[],
	schema: { safeParse: (v: unknown) => { success: boolean } }
): void {
	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(schema.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}
}

function forEachMap<T>(
	cases: MapCase<T>[],
	schema: { safeParse: (v: unknown) => { success: boolean; data?: T } }
): void {
	for (const testCase of cases) {
		it(`maps ${testCase.name}`, () => {
			const parsed = schema.safeParse(testCase.input);
			expect(parsed.success ? parsed.data : null).toBe(testCase.expected);
		});
	}
}

describe('label', () => {
	forEachParse(
		[
			{ name: 'a typical name', input: 'CI runner', ok: true },
			{ name: 'a name at the 120 character limit', input: 'x'.repeat(120), ok: true },
			{ name: 'a padded name, which the transform trims', input: '   padded   ', ok: true },
			{ name: 'an empty name', input: '', ok: false },
			{ name: 'whitespace only', input: '   ', ok: false },
			{ name: 'markup', input: '<script>alert(1)</script>', ok: false },
			{ name: 'one character past the limit', input: 'x'.repeat(121), ok: false }
		],
		label
	);
});

describe('searchText', () => {
	forEachParse(
		[
			{ name: 'a typical search term', input: 'gpt', ok: true },
			{ name: 'a term at the 200 character limit', input: 'q'.repeat(200), ok: true },
			{ name: 'an empty term, which means no filter', input: '', ok: true },
			{ name: 'one character past the limit', input: 'q'.repeat(201), ok: false }
		],
		searchText
	);
});

describe('gatewayKeyInput', () => {
	forEachParse(
		[
			{ name: 'a typical key', input: 'sk-live-0123456789abcdef', ok: true },
			{ name: 'a key with pasted whitespace', input: ' sk-live-0123456789abcdef\n', ok: true },
			{
				name: 'a key at the 20 character floor',
				input: 'sk-0123456789abcdefg',
				ok: true
			},
			{ name: 'a key without the prefix', input: 'live-0123456789abcdef', ok: false },
			{ name: 'a key below the floor', input: 'sk-short', ok: false },
			{ name: 'an empty value', input: '', ok: false }
		],
		gatewayKeyInput
	);
});

describe('secretValue', () => {
	forEachParse(
		[
			{ name: 'a typical secret', input: 'hunter2hunter2', ok: true },
			{ name: 'a secret at the 8 character floor', input: '12345678', ok: true },
			{ name: 'a secret at the 4096 character ceiling', input: 's'.repeat(4096), ok: true },
			{ name: 'a secret below the floor', input: 'short', ok: false },
			{ name: 'a secret past the ceiling', input: 's'.repeat(4097), ok: false }
		],
		secretValue
	);
});

describe('proxyHost', () => {
	forEachParse(
		[
			{ name: 'a typical host', input: 'proxy.example.com', ok: true },
			{
				name: 'a padded host, which the transform normalizes',
				input: '  PROXY.example.com ',
				ok: true
			},
			{ name: 'a host at the 253 character limit', input: `${'h'.repeat(249)}.com`, ok: true },
			{ name: 'an IPv4 literal', input: '10.0.0.7', ok: true },
			// The bracketed form is what the API can use, because it builds `host:port` itself.
			{ name: 'a bracketed IPv6 literal', input: '[2001:db8::1]', ok: true },
			{ name: 'the bracketed loopback', input: '[::1]', ok: true },
			{ name: 'a host with a scheme', input: 'http://proxy.example.com', ok: false },
			{ name: 'a host with a port', input: 'proxy.example.com:8080', ok: false },
			{ name: 'an empty value', input: '', ok: false },
			{ name: 'a host past the limit', input: `${'h'.repeat(250)}.com`, ok: false },
			{ name: 'a bare IPv6 literal, which needs brackets', input: '2001:db8::1', ok: false },
			{ name: 'a bare loopback', input: '::1', ok: false },
			{ name: 'a bracketed literal with a port', input: '[::1]:8080', ok: false }
		],
		proxyHost
	);

	it('names the brackets in the message for a bare IPv6 literal', () => {
		// The two colon cases read differently to an operator, so they say different things: a
		// `host:port` is a host field carrying a port, and a bare literal is a spelling the API
		// cannot use. A single message for both would leave one of them unexplained.
		const bare = proxyHost.safeParse('::1');
		const withPort = proxyHost.safeParse('proxy.example.com:8080');

		expect(bare.success ? '' : bare.error.issues[0]?.message).toBe(
			'Write an IPv6 address in square brackets, for example [::1].'
		);
		expect(withPort.success ? '' : withPort.error.issues[0]?.message).toBe(
			'Enter a host only, with no scheme, path, or port.'
		);
	});
});

describe('proxyPort', () => {
	forEachMap<number>(
		[
			{ name: 'a typical port', input: '8080', expected: 8080 },
			{ name: 'the low boundary', input: '1', expected: 1 },
			{ name: 'the high boundary', input: 65535, expected: 65535 },
			{ name: 'zero', input: '0', expected: null },
			{ name: 'a negative port', input: -1, expected: null },
			{ name: 'one past the high boundary', input: '65536', expected: null },
			{ name: 'a fractional port', input: 80.5, expected: null }
		],
		proxyPort
	);
});

describe('absoluteUrl', () => {
	forEachParse(
		[
			{ name: 'a plain http URL', input: 'http://127.0.0.1:8080', ok: true },
			{ name: 'an https URL with a path', input: 'https://gw.example.com/api', ok: true },
			{
				name: 'a URL with a trailing slash, which the transform removes',
				input: 'http://host:8080/',
				ok: true
			},
			{ name: 'a relative path', input: '/api/v1', ok: false },
			{ name: 'an empty value', input: '', ok: false },
			{ name: 'a non-http scheme', input: 'ftp://host', ok: false },
			{ name: 'text that is not a URL', input: 'not a url', ok: false }
		],
		absoluteUrl
	);
});

describe('noProxyList', () => {
	forEachParse(
		[
			{ name: 'a typical list', input: 'a.com,b.com', ok: true },
			{ name: 'a list with duplicates and blanks', input: 'a.com, ,a.com', ok: true },
			{ name: 'an empty list', input: '', ok: true },
			{ name: 'an entry past the host limit', input: `${'h'.repeat(253)}.com`, ok: false }
		],
		noProxyList
	);
});

describe('perPage', () => {
	forEachMap<number>(
		[
			{ name: 'a typical page size', input: '50', expected: 50 },
			{ name: 'a missing value to the default', input: undefined, expected: 25 },
			{ name: 'the API cap', input: '100', expected: 100 },
			{ name: 'zero', input: '0', expected: null },
			{ name: 'an extreme value above the API cap', input: '100000', expected: null }
		],
		perPage
	);
});
