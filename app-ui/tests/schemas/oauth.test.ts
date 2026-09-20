// Tests for the OAuth contracts (docs/SPEC-API/001-SPEC-API.md §7.4, docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// Each group is a table of input variations, per docs/RULLES/TDD.md §2.5. Two of the rules under test come
// from the gateway's own code: `flowKind` decides the flow, and `domain/oauth_refresh.go` decides the token
// state, with `due` covering both the refresh window and an expiry that has passed. The third is the
// callback's query, which is the one part of this resource the panel reads from an address rather than from
// a response, so it is the one part a stranger can write.

import { describe, expect, it } from 'vitest';
import {
	hasOAuthReturn,
	oauthFlowCopy,
	oauthFlowStartable,
	oauthTokenState,
	parseOAuthReturn,
	schemaOAuthRefresh,
	schemaOAuthRefreshBody,
	schemaOAuthStart,
	schemaOAuthStatus
} from '$lib/schemas/oauth';

const NOW = Date.parse('2026-09-20T12:00:00Z');

describe('oauthFlowCopy', () => {
	const cases = [
		{ name: 'code', flow: 'code', contains: 'can start the authorization' },
		{ name: 'device', flow: 'device', contains: 'device endpoint instead' },
		{ name: 'connector', flow: 'connector', contains: 'needs a connector' },
		{ name: 'none', flow: 'none', contains: 'no authorize URL' },
		{ name: 'a flow the panel does not know', flow: 'hybrid', contains: 'hybrid' }
	];

	for (const testCase of cases) {
		it(`explains ${testCase.name}`, () => {
			expect(oauthFlowCopy(testCase.flow)).toContain(testCase.contains);
		});
	}
});

describe('oauthFlowStartable', () => {
	const cases = [
		{ name: 'code', flow: 'code', want: true },
		{ name: 'device', flow: 'device', want: false },
		{ name: 'connector', flow: 'connector', want: false },
		{ name: 'none', flow: 'none', want: false },
		{ name: 'a flow the panel does not know', flow: 'hybrid', want: false }
	];

	for (const testCase of cases) {
		it(`${testCase.want ? 'offers' : 'refuses'} the start action for ${testCase.name}`, () => {
			expect(oauthFlowStartable(testCase.flow)).toBe(testCase.want);
		});
	}
});

describe('oauthTokenState', () => {
	const cases = [
		{
			name: 'missing, which means no expiry is known',
			state: 'missing',
			expiresAt: null,
			contains: 'No expiry is known'
		},
		{
			name: 'fresh',
			state: 'fresh',
			expiresAt: '2026-09-21T12:00:00Z',
			contains: 'outside its refresh window'
		},
		{
			// `due` covers both cases, so the expiry is what tells them apart.
			name: 'due with an expiry that has passed',
			state: 'due',
			expiresAt: '2026-09-20T11:00:00Z',
			contains: 'has expired'
		},
		{
			name: 'due with an expiry still ahead',
			state: 'due',
			expiresAt: '2026-09-20T13:00:00Z',
			contains: 'inside its refresh window'
		},
		{
			name: 'due with no expiry at all',
			state: 'due',
			expiresAt: null,
			contains: 'inside its refresh window'
		},
		{
			name: 'due with an expiry that cannot be parsed',
			state: 'due',
			expiresAt: 'not-a-date',
			contains: 'inside its refresh window'
		},
		{
			name: 'a state the panel does not know',
			state: 'stale',
			expiresAt: null,
			contains: 'reports this token as stale'
		}
	];

	for (const testCase of cases) {
		it(`reads ${testCase.name}`, () => {
			expect(oauthTokenState(testCase.state, testCase.expiresAt, NOW)).toContain(testCase.contains);
		});
	}

	it('treats an expiry exactly at now as passed', () => {
		expect(oauthTokenState('due', '2026-09-20T12:00:00Z', NOW)).toContain('has expired');
	});
});

describe('parseOAuthReturn', () => {
	const cases = [
		{
			name: 'a connected account',
			query: 'oauth=connected&endpoint_id=ep_1',
			want: { outcome: 'connected', endpointId: 'ep_1' }
		},
		{
			name: 'a connected account with no id',
			query: 'oauth=connected',
			want: { outcome: 'connected', endpointId: null }
		},
		{
			name: 'a failure with the gateway reason',
			query: 'oauth=error&oauth_error=the+state+is+unknown%2C+expired%2C+or+already+used',
			want: { outcome: 'error', reason: 'the state is unknown, expired, or already used' }
		},
		{
			name: 'a failure with no reason',
			query: 'oauth=error',
			want: { outcome: 'error', reason: 'The gateway reported no reason.' }
		},
		{ name: 'an outcome the panel does not know', query: 'oauth=pending', want: null },
		{ name: 'an empty outcome', query: 'oauth=', want: null },
		{ name: 'no oauth key at all', query: 'provider=openai', want: null },
		{ name: 'an empty query', query: '', want: null }
	];

	for (const testCase of cases) {
		it(`reads ${testCase.name}`, () => {
			expect(parseOAuthReturn(new URLSearchParams(testCase.query))).toEqual(testCase.want);
		});
	}

	it('ignores surrounding whitespace on the values', () => {
		expect(parseOAuthReturn(new URLSearchParams('oauth=connected&endpoint_id=+ep_1+'))).toEqual({
			outcome: 'connected',
			endpointId: 'ep_1'
		});
	});
});

describe('hasOAuthReturn', () => {
	const cases = [
		{ name: 'the outcome key', query: 'oauth=connected', want: true },
		{ name: 'the reason key alone', query: 'oauth_error=gone', want: true },
		{ name: 'the endpoint key alone', query: 'endpoint_id=ep_1', want: true },
		{ name: 'no key of ours', query: 'page=2', want: false },
		{ name: 'nothing', query: '', want: false }
	];

	for (const testCase of cases) {
		it(`${testCase.want ? 'finds' : 'does not find'} ${testCase.name}`, () => {
			expect(hasOAuthReturn(new URLSearchParams(testCase.query))).toBe(testCase.want);
		});
	}
});

describe('the read schemas', () => {
	it('accepts an authorize URL over https', () => {
		const parsed = schemaOAuthStart.safeParse({
			authorize_url: 'https://provider.test/authorize?client_id=a',
			state: 'st_1'
		});
		expect(parsed.success).toBe(true);
	});

	const startCases = [
		{ name: 'a javascript URL', url: 'javascript:alert(1)' },
		{ name: 'a relative URL', url: '/authorize' },
		{ name: 'an empty URL', url: '' }
	];

	for (const testCase of startCases) {
		it(`refuses ${testCase.name} as an authorize URL`, () => {
			expect(
				schemaOAuthStart.safeParse({ authorize_url: testCase.url, state: 'st_1' }).success
			).toBe(false);
		});
	}

	it('reads an unknown flow and an unknown token state rather than failing the read', () => {
		const parsed = schemaOAuthStatus.safeParse({
			provider_id: 'xai',
			flow: 'hybrid',
			endpoints: [
				{
					endpoint_id: 'ep_1',
					label: 'xAI',
					status: 'active',
					refresh_state: 'stale'
				}
			]
		});

		expect(parsed.success).toBe(true);
		expect(parsed.success && parsed.data.endpoints[0].expires_at).toBeUndefined();
	});

	it('treats absent endpoints as none connected', () => {
		const parsed = schemaOAuthStatus.safeParse({ provider_id: 'xai', flow: 'code' });
		expect(parsed.success && parsed.data.endpoints).toEqual([]);
	});

	it('treats an absent endpoint list on a refresh as nothing moved', () => {
		const parsed = schemaOAuthRefresh.safeParse({ refreshed: 0 });
		expect(parsed.success && parsed.data.endpoint_ids).toEqual([]);
	});

	it('refuses a refresh body with a field the API does not read', () => {
		expect(schemaOAuthRefreshBody.safeParse({ endpoint_id: 'ep_1', force: true }).success).toBe(
			false
		);
	});
});
