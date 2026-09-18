// Routing derivation tests.
//
// SPEC-UI §6.2 asks the endpoint drawer to name the key the router would spend, derived from the fields
// the API returns rather than re-simulated. That makes this function the panel's answer to "why did my
// request fail over", so the cases below are the ones an operator would actually ask about: a healthy
// primary, a disabled primary, an unhealthy primary, a tie, and nothing usable at all.

import { describe, expect, it } from 'vitest';
import type { EndpointKey } from '$lib/schemas/endpoint';
import { isLastActiveApiKey, routingKey } from '$lib/utils/routing';

// Builds a key from the API's own vocabulary, so a case only states what it is about.
function key(overrides: Partial<EndpointKey> & { id: string }): EndpointKey {
	return {
		endpoint_id: 'ep_primary',
		label: 'primary',
		key_hint: 'sk-...abcd',
		priority: 1,
		status: 'active',
		available: true,
		consecutive_errors: 0,
		created_at: '2026-09-01T00:00:00Z',
		updated_at: '2026-09-01T00:00:00Z',
		...overrides
	};
}

describe('routingKey', () => {
	const CASES: { keys: EndpointKey[]; expected: string | null; why: string }[] = [
		{ keys: [], expected: null, why: 'an endpoint with no keys routes nowhere' },
		{ keys: [key({ id: 'uky_a' })], expected: 'uky_a', why: 'the only key' },
		{
			keys: [key({ id: 'uky_a', priority: 2 }), key({ id: 'uky_b', priority: 1 })],
			expected: 'uky_b',
			why: 'the lower priority number wins regardless of order'
		},
		{
			keys: [key({ id: 'uky_a', priority: 5 }), key({ id: 'uky_b', priority: 5 })],
			expected: 'uky_a',
			why: 'a tie keeps the first key in the API order'
		},
		{
			keys: [key({ id: 'uky_a', status: 'disabled' }), key({ id: 'uky_b', priority: 9 })],
			expected: 'uky_b',
			why: 'a disabled key is skipped even at a better priority'
		},
		{
			keys: [key({ id: 'uky_a', available: false }), key({ id: 'uky_b', priority: 9 })],
			expected: 'uky_b',
			why: 'an unhealthy key is skipped even at a better priority'
		},
		{
			keys: [key({ id: 'uky_a', status: 'disabled' }), key({ id: 'uky_b', available: false })],
			expected: null,
			why: 'nothing usable means no key, not a fallback to an unusable one'
		}
	];

	for (const testCase of CASES) {
		it(`picks ${testCase.expected ?? 'nothing'} when ${testCase.why}`, () => {
			expect(routingKey(testCase.keys)?.id ?? null).toBe(testCase.expected);
		});
	}

	it('returns the key itself, so the drawer can show its label and hint', () => {
		const chosen = routingKey([
			key({ id: 'uky_a', label: 'work account', key_hint: 'sk-...wxyz' })
		]);

		expect(chosen?.label).toBe('work account');
		expect(chosen?.key_hint).toBe('sk-...wxyz');
	});
});

describe('isLastActiveApiKey', () => {
	const CASES: {
		keys: EndpointKey[];
		keyId: string;
		authType: string;
		expected: boolean;
		why: string;
	}[] = [
		{
			keys: [key({ id: 'uky_a' })],
			keyId: 'uky_a',
			authType: 'api_key',
			expected: true,
			why: 'the one active key of an api_key endpoint cannot be deleted'
		},
		{
			keys: [key({ id: 'uky_a' })],
			keyId: 'uky_a',
			authType: 'apikey',
			expected: true,
			why: 'the registry spelling means the same credential'
		},
		{
			keys: [key({ id: 'uky_a' })],
			keyId: 'uky_a',
			authType: 'oauth',
			expected: false,
			why: 'an OAuth endpoint has no key to be left without'
		},
		{
			keys: [key({ id: 'uky_a' })],
			keyId: 'uky_a',
			authType: 'no_auth',
			expected: false,
			why: 'an endpoint needing no credential is never refused'
		},
		{
			keys: [key({ id: 'uky_a' }), key({ id: 'uky_b', priority: 2 })],
			keyId: 'uky_a',
			authType: 'api_key',
			expected: false,
			why: 'a second active key means the endpoint still routes'
		},
		{
			keys: [key({ id: 'uky_a' }), key({ id: 'uky_b', status: 'disabled', priority: 2 })],
			keyId: 'uky_a',
			authType: 'api_key',
			expected: true,
			why: 'a disabled sibling does not count as active'
		},
		{
			keys: [key({ id: 'uky_a' }), key({ id: 'uky_b', status: 'disabled', priority: 2 })],
			keyId: 'uky_b',
			authType: 'api_key',
			expected: false,
			why: 'the disabled key is not the last active one, so deleting it is allowed'
		},
		{
			keys: [key({ id: 'uky_a' })],
			keyId: 'uky_other',
			authType: 'api_key',
			expected: false,
			why: 'a different key is not the last active one'
		}
	];

	for (const testCase of CASES) {
		it(`returns ${testCase.expected} when ${testCase.why}`, () => {
			expect(isLastActiveApiKey(testCase.keys, testCase.keyId, testCase.authType)).toBe(
				testCase.expected
			);
		});
	}
});
