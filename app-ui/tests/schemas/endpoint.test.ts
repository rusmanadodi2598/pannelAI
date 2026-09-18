// Upstream endpoint schema contract tests.
//
// These are the tests that catch a drift between docs/SPEC-API/001-SPEC-API.md §7.5 and the panel: every
// field the Go DTO marks `omitempty` has to be accepted when it is absent, and every field the API bounds
// has to be refused outside that bound here rather than by the server after a round trip. The auth
// spellings are covered explicitly because the API accepts the registry's `apikey` and `none` as well as
// its own, and a panel that narrowed the set would fail to parse what the API just returned.

import { describe, expect, it } from 'vitest';
import {
	AUTH_TYPES,
	ENDPOINT_STATUS_ACTIVE,
	schemaAddEndpointKeyForm,
	schemaBulkAddKeysForm,
	schemaCreateEndpointForm,
	schemaEndpoint,
	schemaEndpointKey,
	schemaEndpointList,
	schemaUpdateEndpointForm,
	schemaUpdateEndpointKeyForm
} from '$lib/schemas/endpoint';

// A complete endpoint as the API returns it, so a case only states the field it is about.
function endpoint(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'ep_primary',
		provider_id: 'openai',
		label: 'Primary',
		auth_type: 'api_key',
		priority: 1,
		status: 'active',
		account: {},
		key_count: 2,
		active_key_count: 2,
		available: true,
		created_at: '2026-09-01T00:00:00Z',
		updated_at: '2026-09-01T00:00:00Z',
		...overrides
	};
}

function key(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'uky_a',
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

describe('schemaEndpoint', () => {
	it('accepts a full payload', () => {
		const result = schemaEndpoint.safeParse(
			endpoint({
				provider_name: 'OpenAI',
				oauth: {
					expires_at: '2026-10-01T00:00:00Z',
					scopes: ['read'],
					has_access_token: true,
					has_refresh_token: false
				},
				test_status: { state: 'pass', latency_ms: 120, checked_at: '2026-09-18T00:00:00Z' },
				rate_limited_until: '2026-09-18T01:00:00Z',
				last_used_at: '2026-09-18T00:00:00Z',
				keys: [key()]
			})
		);

		expect(result.success).toBe(true);
	});

	it('accepts a payload with every omitempty field absent', () => {
		// Go omits a nil pointer, so these arrive missing rather than null on a fresh endpoint.
		const result = schemaEndpoint.safeParse(endpoint());

		expect(result.success).toBe(true);
	});

	it('accepts an explicit null for the timestamps the API may send as null', () => {
		const result = schemaEndpoint.safeParse(
			endpoint({ last_used_at: null, rate_limited_until: null })
		);

		expect(result.success).toBe(true);
	});

	it('accepts a status and auth type the panel does not recognise', () => {
		// Rendering an unknown value verbatim is the documented behaviour (§14 Q9), so it must parse.
		const result = schemaEndpoint.safeParse(endpoint({ status: 'quarantined', auth_type: 'mtls' }));

		expect(result.success).toBe(true);
	});

	const INVALID: { payload: Record<string, unknown>; why: string }[] = [
		{ payload: endpoint({ id: 'gky_wrong' }), why: 'an id without the ep_ prefix' },
		{ payload: endpoint({ key_count: '2' }), why: 'a count sent as a string' },
		{ payload: endpoint({ available: 'true' }), why: 'a boolean sent as a string' },
		{ payload: endpoint({ priority: 0 }), why: 'a priority below the API minimum' },
		{ payload: endpoint({ priority: 10001 }), why: 'a priority above the API maximum' },
		{ payload: endpoint({ priority: 1.5 }), why: 'a fractional priority' },
		{ payload: endpoint({ created_at: 'not a date' }), why: 'an unparsable timestamp' }
	];

	for (const testCase of INVALID) {
		it(`rejects ${testCase.why}`, () => {
			expect(schemaEndpoint.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('schemaEndpointKey', () => {
	it('accepts a full payload including health fields', () => {
		const result = schemaEndpointKey.safeParse(
			key({
				last_used_at: '2026-09-18T00:00:00Z',
				last_error: 'upstream 429',
				consecutive_errors: 3,
				rate_limited_until: '2026-09-18T01:00:00Z'
			})
		);

		expect(result.success).toBe(true);
	});

	it('accepts a key that has never been used', () => {
		expect(schemaEndpointKey.safeParse(key()).success).toBe(true);
	});

	it('rejects an id without the uky_ prefix', () => {
		expect(schemaEndpointKey.safeParse(key({ id: 'ep_wrong' })).success).toBe(false);
	});

	it('rejects a negative consecutive error count', () => {
		expect(schemaEndpointKey.safeParse(key({ consecutive_errors: -1 })).success).toBe(false);
	});
});

describe('schemaEndpointList', () => {
	it('requires the pagination block', () => {
		expect(schemaEndpointList.safeParse({ data: [endpoint()] }).success).toBe(false);
	});

	it('accepts an empty page', () => {
		const result = schemaEndpointList.safeParse({
			data: [],
			meta: { page: 1, per_page: 25, total: 0 }
		});

		expect(result.success).toBe(true);
	});
});

describe('schemaCreateEndpointForm', () => {
	for (const authType of AUTH_TYPES) {
		it(`accepts the ${authType} auth type the API publishes`, () => {
			const result = schemaCreateEndpointForm.safeParse({
				provider_id: 'openai',
				label: 'Primary',
				auth_type: authType
			});

			expect(result.success).toBe(true);
		});
	}

	it('accepts an omitted first key, because an endpoint may be created empty', () => {
		const result = schemaCreateEndpointForm.safeParse({
			provider_id: 'openai',
			label: 'Primary',
			auth_type: 'api_key',
			key_value: ''
		});

		expect(result.success).toBe(true);
	});

	const INVALID: { payload: Record<string, unknown>; why: string }[] = [
		{
			payload: { provider_id: '  ', label: 'Primary', auth_type: 'api_key' },
			why: 'a blank provider'
		},
		{ payload: { provider_id: 'openai', label: '', auth_type: 'api_key' }, why: 'a blank label' },
		{
			payload: { provider_id: 'openai', label: 'Primary', auth_type: 'cookie' },
			why: 'the cookie auth type, which has no v1 transport'
		},
		{
			payload: { provider_id: 'openai', label: 'Primary', auth_type: 'api_key', priority: 0 },
			why: 'a priority below the API minimum'
		},
		{
			payload: {
				provider_id: 'openai',
				label: 'Primary',
				auth_type: 'api_key',
				key_value: 'short'
			},
			why: 'a credential shorter than the panel minimum'
		}
	];

	for (const testCase of INVALID) {
		it(`rejects ${testCase.why}`, () => {
			expect(schemaCreateEndpointForm.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('schemaUpdateEndpointForm', () => {
	it('accepts a priority-only change, which is what reorders siblings', () => {
		const result = schemaUpdateEndpointForm.safeParse({ priority: 3 });

		expect(result.success).toBe(true);
	});

	it('accepts a status change to disabled', () => {
		expect(schemaUpdateEndpointForm.safeParse({ status: ENDPOINT_STATUS_ACTIVE }).success).toBe(
			true
		);
	});

	it('rejects a status outside the two the API accepts on a patch', () => {
		expect(schemaUpdateEndpointForm.safeParse({ status: 'quarantined' }).success).toBe(false);
	});

	it('rejects an unknown field, because the API rejects it too', () => {
		expect(schemaUpdateEndpointForm.safeParse({ provider_id: 'anthropic' }).success).toBe(false);
	});
});

describe('schemaAddEndpointKeyForm', () => {
	it('accepts a key with only a value', () => {
		expect(schemaAddEndpointKeyForm.safeParse({ value: 'sk-abcdefgh' }).success).toBe(true);
	});

	it('rejects a value that is too short to be a credential', () => {
		expect(schemaAddEndpointKeyForm.safeParse({ value: 'sk-' }).success).toBe(false);
	});

	it('rejects a missing value, because a key is the point of the form', () => {
		expect(schemaAddEndpointKeyForm.safeParse({ label: 'backup' }).success).toBe(false);
	});
});

describe('schemaUpdateEndpointKeyForm', () => {
	it('accepts a label-only edit, which leaves the stored credential alone', () => {
		expect(schemaUpdateEndpointKeyForm.safeParse({ label: 'renamed' }).success).toBe(true);
	});

	it('accepts an empty value as "keep the stored credential"', () => {
		expect(schemaUpdateEndpointKeyForm.safeParse({ value: '' }).success).toBe(true);
	});

	it('rejects a replacement value that is too short', () => {
		expect(schemaUpdateEndpointKeyForm.safeParse({ value: 'sk-' }).success).toBe(false);
	});
});

describe('schemaBulkAddKeysForm', () => {
	it('accepts the API batch maximum', () => {
		const keys = Array.from({ length: 100 }, (_, index) => ({ value: `sk-abcdefgh${index}` }));

		expect(schemaBulkAddKeysForm.safeParse({ keys }).success).toBe(true);
	});

	it('rejects an empty batch', () => {
		expect(schemaBulkAddKeysForm.safeParse({ keys: [] }).success).toBe(false);
	});

	it('rejects a batch over the API maximum rather than letting the server refuse it', () => {
		const keys = Array.from({ length: 101 }, (_, index) => ({ value: `sk-abcdefgh${index}` }));

		expect(schemaBulkAddKeysForm.safeParse({ keys }).success).toBe(false);
	});

	it('rejects a batch where one row is invalid, naming no row but refusing the whole batch', () => {
		const result = schemaBulkAddKeysForm.safeParse({
			keys: [{ value: 'sk-abcdefgh' }, { value: 'bad' }]
		});

		expect(result.success).toBe(false);
	});
});
