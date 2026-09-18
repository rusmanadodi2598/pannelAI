// Provider schema contract tests.
//
// The registry is the panel's read-only view of what the gateway can route to, so these tests pin the two
// things that would silently break the screen: a Go nil slice arriving as `null` rather than `[]` (which
// happens for every list field the DTO does not mark `omitempty`), and the status summary being reduced to
// the one sentence the row shows. The sentence is what an operator actually reads, so it is tested directly
// rather than left to the template.

import { describe, expect, it } from 'vitest';
import {
	schemaProvider,
	schemaProviderDetail,
	schemaProviderList,
	schemaProviderQuery,
	statusSummaryText
} from '$lib/schemas/provider';

function provider(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'openai',
		name: 'OpenAI',
		category: 'apikey',
		auth_type: 'apikey',
		auth_modes: ['apikey'],
		has_oauth: false,
		no_auth: false,
		routability: 'native',
		endpoint_count: 2,
		status_summary: { total: 2, active: 2, disabled: 0, error: 0, rate_limited: 0 },
		...overrides
	};
}

describe('schemaProvider', () => {
	it('accepts a full row', () => {
		expect(schemaProvider.safeParse(provider()).success).toBe(true);
	});

	it('accepts auth_modes as null, which is how Go marshals a nil slice', () => {
		const result = schemaProvider.safeParse(provider({ auth_modes: null }));

		expect(result.success).toBe(true);
		expect(result.success && result.data.auth_modes).toEqual([]);
	});

	it('accepts an omitted auth_modes, because the field is not required to be present', () => {
		const payload = provider();
		delete payload.auth_modes;

		const result = schemaProvider.safeParse(payload);

		expect(result.success).toBe(true);
		expect(result.success && result.data.auth_modes).toEqual([]);
	});

	it('accepts a category the panel does not enumerate', () => {
		// Category is an open string in the reference registry, so an unknown value renders verbatim.
		expect(schemaProvider.safeParse(provider({ category: 'aggregator' })).success).toBe(true);
	});

	const INVALID: { payload: Record<string, unknown>; why: string }[] = [
		{ payload: provider({ id: '' }), why: 'an empty provider id' },
		{ payload: provider({ name: '' }), why: 'an empty display name' },
		{ payload: provider({ endpoint_count: -1 }), why: 'a negative endpoint count' },
		{ payload: provider({ has_oauth: 'false' }), why: 'a boolean sent as a string' },
		{ payload: provider({ status_summary: { total: 0 } }), why: 'a partial status summary' }
	];

	for (const testCase of INVALID) {
		it(`rejects ${testCase.why}`, () => {
			expect(schemaProvider.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('schemaProviderList', () => {
	it('requires the pagination block, because the registry is a paged set', () => {
		expect(schemaProviderList.safeParse({ data: [provider()] }).success).toBe(false);
	});

	it('accepts an empty page', () => {
		const result = schemaProviderList.safeParse({
			data: [],
			meta: { page: 1, per_page: 25, total: 0 }
		});

		expect(result.success).toBe(true);
	});
});

describe('schemaProviderDetail', () => {
	it('accepts the list fields plus the transport defaults', () => {
		const result = schemaProviderDetail.safeParse(
			provider({
				base_url: 'https://api.openai.com',
				format: 'openai',
				url_suffix: '/v1/chat/completions',
				validate_url: '/v1/models',
				timeout_ms: 30000,
				model_count: 12,
				chat_model_count: 9,
				media: [],
				deprecated: false
			})
		);

		expect(result.success).toBe(true);
	});

	it('accepts media as null and normalizes it to an empty list', () => {
		const payload = provider({
			base_url: '',
			format: 'openai',
			url_suffix: '',
			validate_url: '',
			timeout_ms: 30000,
			model_count: 0,
			chat_model_count: 0,
			media: null,
			deprecated: false
		});

		const result = schemaProviderDetail.safeParse(payload);

		expect(result.success).toBe(true);
		expect(result.success && result.data.media).toEqual([]);
	});
});

describe('statusSummaryText', () => {
	const CASES: {
		summary: {
			total: number;
			active: number;
			disabled: number;
			error: number;
			rate_limited: number;
		};
		expected: string;
		why: string;
	}[] = [
		{
			summary: { total: 0, active: 0, disabled: 0, error: 0, rate_limited: 0 },
			expected: 'No endpoint configured',
			why: 'nothing is configured for this provider'
		},
		{
			summary: { total: 2, active: 2, disabled: 0, error: 0, rate_limited: 0 },
			expected: '2 active',
			why: 'every endpoint is healthy'
		},
		{
			summary: { total: 3, active: 1, disabled: 0, error: 2, rate_limited: 0 },
			expected: '2 failing',
			why: 'failures outrank the healthy count, because they need attention'
		},
		{
			summary: { total: 2, active: 1, disabled: 0, error: 0, rate_limited: 1 },
			expected: '1 rate limited',
			why: 'a rate limit outranks the healthy count too'
		},
		{
			summary: { total: 2, active: 0, disabled: 2, error: 0, rate_limited: 0 },
			expected: '2 disabled',
			why: 'everything switched off'
		},
		{
			summary: { total: 1, active: 1, disabled: 0, error: 1, rate_limited: 1 },
			expected: '1 failing',
			why: 'a failure is reported before a rate limit'
		}
	];

	for (const testCase of CASES) {
		it(`says "${testCase.expected}" when ${testCase.why}`, () => {
			expect(statusSummaryText(testCase.summary)).toBe(testCase.expected);
		});
	}
});

describe('schemaProviderQuery', () => {
	it('accepts a category filter', () => {
		expect(schemaProviderQuery.safeParse({ category: 'oauth' }).success).toBe(true);
	});

	it('accepts the two routability values the API validates', () => {
		expect(schemaProviderQuery.safeParse({ routability: 'native' }).success).toBe(true);
		expect(schemaProviderQuery.safeParse({ routability: 'connector' }).success).toBe(true);
	});

	it('rejects a routability value the API would refuse, so the panel fails before the round trip', () => {
		expect(schemaProviderQuery.safeParse({ routability: 'gateway' }).success).toBe(false);
	});

	it('rejects a search parameter, because the API accepts none', () => {
		// This is deliberate: if the API grows `search`, this test is where the panel learns it can send one.
		expect(schemaProviderQuery.safeParse({ search: 'open' }).success).toBe(false);
	});
});
