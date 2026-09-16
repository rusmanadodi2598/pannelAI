// Tests for the /api/v1 forwarding rules.
//
// Forwarding is where a session cookie is won or lost, so the rules are asserted rather than assumed:
// which paths are forwarded, which headers are dropped, and how the target URL is built. Every rule
// is a row, so a header added to the hop-by-hop list is a row that changes rather than a rule that
// silently stops being checked.

import { describe, expect, it } from 'vitest';
import { buildForwardHeaders, buildForwardUrl, shouldProxy } from '$lib/server/proxy';

const target = new URL('http://up:8080');

describe('shouldProxy', () => {
	const cases = [
		{ name: 'the prefix itself', path: '/api/v1', expected: true },
		{ name: 'a management route', path: '/api/v1/gateway-keys', expected: true },
		{ name: 'a nested route', path: '/api/v1/endpoints/ep_1/keys', expected: true },
		{ name: 'the prefix with a query', path: '/api/v1', expected: true },
		{ name: 'a panel route', path: '/endpoint-keys', expected: false },
		{ name: 'the root', path: '/', expected: false },
		{ name: 'a lookalike prefix', path: '/api/v10/gateway-keys', expected: false },
		{ name: 'a data-plane path outside v1', path: '/api/v1beta/models', expected: false },
		{ name: 'an empty path', path: '', expected: false }
	];

	for (const testCase of cases) {
		it(`${testCase.expected ? 'forwards' : 'ignores'} ${testCase.name}`, () => {
			expect(shouldProxy(testCase.path)).toBe(testCase.expected);
		});
	}
});

describe('buildForwardUrl', () => {
	const cases = [
		{
			name: 'a plain path',
			target: 'http://up:8080',
			path: '/api/v1/health',
			search: '',
			expected: 'http://up:8080/api/v1/health'
		},
		{
			name: 'a query string',
			target: 'http://up:8080',
			path: '/api/v1/providers',
			search: '?category=oauth&page=2',
			expected: 'http://up:8080/api/v1/providers?category=oauth&page=2'
		},
		{
			name: 'an encoded query value',
			target: 'http://up:8080',
			path: '/api/v1/logs',
			search: '?search=a%20b&page=2',
			expected: 'http://up:8080/api/v1/logs?search=a%20b&page=2'
		},
		{
			name: 'an id in the path',
			target: 'http://up:8080',
			path: '/api/v1/gateway-keys/gky_1',
			search: '',
			expected: 'http://up:8080/api/v1/gateway-keys/gky_1'
		},
		{
			name: 'a path when the target carries its own path',
			target: 'http://up:8080/ignored',
			path: '/api/v1/health',
			search: '',
			expected: 'http://up:8080/api/v1/health'
		}
	];

	for (const testCase of cases) {
		it(`builds ${testCase.name}`, () => {
			const request = new URL(`http://panel.local${testCase.path}${testCase.search}`);
			expect(buildForwardUrl(new URL(testCase.target), request).toString()).toBe(testCase.expected);
		});
	}
});

type HeaderCase = {
	name: string;
	incoming: Record<string, string>;
	expected: Record<string, string | null>;
};

describe('buildForwardHeaders', () => {
	const cases: HeaderCase[] = [
		{
			name: 'keeps the session cookie',
			incoming: { cookie: 'pannel_session=abc' },
			expected: { cookie: 'pannel_session=abc' }
		},
		{
			name: 'keeps accept',
			incoming: { accept: 'application/json' },
			expected: { accept: 'application/json' }
		},
		{
			name: 'drops connection',
			incoming: { connection: 'keep-alive' },
			expected: { connection: null }
		},
		{
			name: 'drops keep-alive',
			incoming: { 'keep-alive': 'timeout=5' },
			expected: { 'keep-alive': null }
		},
		{
			name: 'drops content-length',
			incoming: { 'content-length': '42' },
			expected: { 'content-length': null }
		},
		{
			name: 'drops transfer-encoding',
			incoming: { 'transfer-encoding': 'chunked' },
			expected: { 'transfer-encoding': null }
		},
		{ name: 'drops upgrade', incoming: { upgrade: 'websocket' }, expected: { upgrade: null } },
		{ name: 'drops te', incoming: { te: 'trailers' }, expected: { te: null } },
		{ name: 'drops trailer', incoming: { trailer: 'expires' }, expected: { trailer: null } },
		{
			name: 'drops proxy-authorization',
			incoming: { 'proxy-authorization': 'Basic xyz' },
			expected: { 'proxy-authorization': null }
		},
		{
			name: 'drops proxy-authenticate',
			incoming: { 'proxy-authenticate': 'Basic' },
			expected: { 'proxy-authenticate': null }
		},
		{
			name: 'replaces the incoming host with the upstream host',
			incoming: { host: 'panel.local' },
			expected: { host: 'up:8080' }
		},
		{
			name: 'sets x-forwarded-host from the referer',
			incoming: { referer: 'https://panel.local/endpoint-keys' },
			expected: { 'x-forwarded-host': 'panel.local' }
		},
		{
			name: 'falls back to the upstream host when there is no referer',
			incoming: {},
			expected: { 'x-forwarded-host': 'up:8080' }
		}
	];

	for (const testCase of cases) {
		it(testCase.name, () => {
			const forwarded = buildForwardHeaders(new Headers(testCase.incoming), target);

			for (const [name, expected] of Object.entries(testCase.expected)) {
				expect(forwarded.get(name), `${name} forwarded as ${expected}`).toBe(expected);
			}
		});
	}

	it('drops every hop-by-hop header except the host it replaces', () => {
		const hopByHop = [
			'connection',
			'keep-alive',
			'proxy-authenticate',
			'proxy-authorization',
			'te',
			'trailer',
			'transfer-encoding',
			'upgrade',
			'host',
			'content-length'
		];
		const entries: [string, string][] = hopByHop.map((name) => [name, 'value']);
		const forwarded = buildForwardHeaders(new Headers(entries), target);

		for (const name of hopByHop) {
			if (name === 'host') {
				expect(forwarded.get('host'), 'host must be replaced, not dropped').toBe('up:8080');
				continue;
			}

			expect(forwarded.get(name), `${name} must not be forwarded`).toBeNull();
		}
	});
});
