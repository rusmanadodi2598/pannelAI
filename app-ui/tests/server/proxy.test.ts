// Tests for the /api/v1 forwarding rules.
//
// Forwarding is where a session cookie is won or lost, so the rules are asserted rather than assumed:
// which paths are forwarded, which headers are dropped, and how the target URL is built.

import { describe, expect, it } from 'vitest';
import { buildForwardHeaders, buildForwardUrl, shouldProxy } from '$lib/server/proxy';

describe('shouldProxy', () => {
	const cases = [
		{ name: 'the prefix itself', path: '/api/v1', expected: true },
		{ name: 'a management route', path: '/api/v1/gateway-keys', expected: true },
		{ name: 'a nested route', path: '/api/v1/endpoints/ep_1/keys', expected: true },
		{ name: 'a panel route', path: '/endpoint-keys', expected: false },
		{ name: 'the root', path: '/', expected: false },
		{ name: 'a lookalike prefix', path: '/api/v10/gateway-keys', expected: false },
		{ name: 'a data-plane path outside v1', path: '/api/v1beta/models', expected: false }
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
			path: '/api/v1/health',
			search: '',
			expected: 'http://up:8080/api/v1/health'
		},
		{
			name: 'a query string',
			path: '/api/v1/providers',
			search: '?category=oauth&page=2',
			expected: 'http://up:8080/api/v1/providers?category=oauth&page=2'
		},
		{
			name: 'an encoded id',
			path: '/api/v1/gateway-keys/gky_1',
			search: '',
			expected: 'http://up:8080/api/v1/gateway-keys/gky_1'
		}
	];

	for (const testCase of cases) {
		it(`builds ${testCase.name}`, () => {
			const target = new URL('http://up:8080');
			const request = new URL(`http://panel.local${testCase.path}${testCase.search}`);
			expect(buildForwardUrl(target, request).toString()).toBe(testCase.expected);
		});
	}
});

describe('buildForwardHeaders', () => {
	const target = new URL('http://up:8080');

	it('keeps the session cookie and drops hop-by-hop headers', () => {
		const incoming = new Headers({
			cookie: 'pannel_session=abc',
			accept: 'application/json',
			connection: 'keep-alive',
			'content-length': '42',
			'transfer-encoding': 'chunked',
			host: 'panel.local'
		});

		const forwarded = buildForwardHeaders(incoming, target);

		expect(forwarded.get('cookie')).toBe('pannel_session=abc');
		expect(forwarded.get('accept')).toBe('application/json');
		expect(forwarded.get('connection')).toBeNull();
		expect(forwarded.get('content-length')).toBeNull();
		expect(forwarded.get('transfer-encoding')).toBeNull();
	});

	it('sets the upstream host', () => {
		const forwarded = buildForwardHeaders(new Headers({ host: 'panel.local' }), target);
		expect(forwarded.get('host')).toBe('up:8080');
	});
});
