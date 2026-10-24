// Routing derivation tests.
//
// SPEC-UI §6.2 asks the endpoint drawer to name the key the router would spend, derived from the fields the
// API returns rather than re-simulated, and to say why a key is out of rotation. Both are derivation, so
// both live here: `routingKey` and `isLastActiveApiKey` answer which key the walk spends and which one the
// API would refuse to delete, and `parkProvenance` answers what a parked key failed as. The cases are the
// ones an operator would actually ask about: a healthy primary, a disabled primary, an unhealthy primary, a
// tie, nothing usable at all, and a wait the vendor warranted rather than one the gateway guessed.

import { describe, expect, it } from 'vitest';
import type { EndpointKey } from '$lib/schemas/endpoint';
import { isLastActiveApiKey, parkProvenance, routingKey } from '$lib/utils/routing';
import { forEachCase } from '../support/tables';

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
			authType: 'service_account',
			expected: true,
			why: 'the service-account JSON is the last credential the endpoint mints a bearer from'
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

// The parked key's one line: what it failed as, and whose wait the countdown is showing.
//
// `rate_limited_until` alone cannot tell a throttle the vendor expects to lift from an allowance spent
// until a date the vendor printed in its own body, which is the confusion the classifier exists to end.
// The cases below are the answers an operator reads in the drawer, including the two that read as nothing:
// a key that never failed, and a key parked before the classifier existed, both of which the API leaves
// unset rather than filled with a guess.
describe('parkProvenance', () => {
	const CASES: { key: EndpointKey; expected: string; why: string }[] = [
		{
			key: key({ id: 'uky_a' }),
			expected: '',
			why: 'a key that names no class has no cause to read'
		},
		{
			key: key({ id: 'uky_a', last_error_class: 'QUOTA_EXHAUSTED' }),
			expected: 'parked as Quota exhausted',
			why: 'a class with no source recorded names the class only'
		},
		{
			key: key({ id: 'uky_a', last_error_class: 'QUOTA_EXHAUSTED', reset_source: 'body' }),
			expected: "parked as Quota exhausted, wait named in the vendor's body",
			why: "a reset the vendor stated in its body is the vendor's"
		},
		{
			key: key({ id: 'uky_a', last_error_class: 'RATE_LIMIT_TEMPORARY', reset_source: 'header' }),
			expected: "parked as Temporary rate limit, wait from the vendor's Retry-After header",
			why: 'a Retry-After header is the vendor stating the wait another way'
		},
		{
			key: key({ id: 'uky_a', last_error_class: 'AUTH_INVALID', reset_source: 'default' }),
			expected: 'parked as Rejected credential, wait chosen by the gateway',
			why: 'a default source is the gateway admitting it guessed'
		},
		{
			key: key({
				id: 'uky_a',
				last_error_class: 'UNKNOWN',
				reset_source: 'default',
				signature_id: 'tokenharbor.free-allowance'
			}),
			expected:
				'parked as Unrecognised failure, wait chosen by the gateway (tokenharbor.free-allowance)',
			why: 'the signature rides at the end, because it is what an operator quotes'
		},
		{
			key: key({ id: 'uky_a', last_error_class: 'ratelimit', reset_source: 'body' }),
			expected: "parked as ratelimit, wait named in the vendor's body",
			why: 'a legacy parking token has no label, so it reads as itself'
		},
		{
			key: key({ id: 'uky_a', last_error_class: 'BILLING_EXHAUSTED', reset_source: 'sideways' }),
			expected: 'parked as Billing exhausted, sideways',
			why: 'a source the panel has never seen renders rather than blanking the row'
		}
	];

	forEachCase(
		CASES.map((testCase) => ({ ...testCase, name: testCase.why })),
		(testCase) => {
			expect(parkProvenance(testCase.key)).toBe(testCase.expected);
		}
	);
});
