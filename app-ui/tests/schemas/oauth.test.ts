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
	oauthDeviceConnected,
	oauthFlowCopy,
	oauthFlowDevice,
	oauthFlowStartable,
	oauthTokenState,
	parseOAuthReturn,
	schemaOAuthDevicePoll,
	schemaOAuthDevicePollBody,
	schemaOAuthDeviceStart,
	schemaOAuthRefresh,
	schemaOAuthRefreshBody,
	schemaOAuthStart,
	schemaOAuthStatus
} from '$lib/schemas/oauth';

const NOW = Date.parse('2026-09-20T12:00:00Z');

describe('oauthFlowCopy', () => {
	const cases = [
		{ name: 'code', flow: 'code', contains: 'can start the authorization' },
		{ name: 'device', flow: 'device', contains: 'device page' },
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

describe('oauthFlowDevice', () => {
	const cases = [
		{ name: 'device', flow: 'device', want: true },
		{ name: 'code', flow: 'code', want: false },
		{ name: 'connector', flow: 'connector', want: false },
		{ name: 'none', flow: 'none', want: false },
		{ name: 'a flow the panel does not know', flow: 'hybrid', want: false }
	];

	for (const testCase of cases) {
		it(`${testCase.want ? 'offers' : 'refuses'} a device round for ${testCase.name}`, () => {
			expect(oauthFlowDevice(testCase.flow)).toBe(testCase.want);
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

	it('reads a device round and treats its URL as the vendor address it is', () => {
		const parsed = schemaOAuthDeviceStart.safeParse({
			device_code: 'dev_1',
			verification_url: 'https://qoder.test/device/selectAccounts?challenge=c1',
			user_code: 'AB12CD34',
			interval_seconds: 2,
			expires_in: 300
		});
		expect(parsed.success).toBe(true);
	});

	// A vendor-minted state round has no short code at all: the vendor hands out a state
	// and an authorization page, and there is nothing for the operator to recognize. The
	// round is still startable and pollable, so the absence is read, not refused.
	it('reads a vendor-minted round that carries no short code', () => {
		const parsed = schemaOAuthDeviceStart.safeParse({
			device_code: 'vendor-state-7f3a',
			verification_url: 'https://www.codebuddy.test/auth?state=vendor-state-7f3a',
			interval_seconds: 5,
			expires_in: 300
		});
		expect(parsed.success && parsed.data.user_code).toBeUndefined();
		expect(parsed.success && parsed.data.device_code).toBe('vendor-state-7f3a');
	});

	it('refuses a device round whose short code is present but empty', () => {
		expect(
			schemaOAuthDeviceStart.safeParse({
				device_code: 'dev_1',
				verification_url: 'https://qoder.test/device/selectAccounts',
				user_code: '',
				interval_seconds: 2,
				expires_in: 300
			}).success
		).toBe(false);
	});

	const deviceStartCases = [
		{ name: 'a script URL as the device page', patch: { verification_url: 'javascript:alert(1)' } },
		{ name: 'a relative device page', patch: { verification_url: '/device/selectAccounts' } },
		{ name: 'no device code', patch: { device_code: '' } },
		{ name: 'an interval past a minute', patch: { interval_seconds: 61 } },
		{ name: 'a round that never expires', patch: { expires_in: 0 } },
		{ name: 'a fractional interval', patch: { interval_seconds: 0.5 } }
	];

	for (const testCase of deviceStartCases) {
		it(`refuses ${testCase.name} on a device round`, () => {
			expect(
				schemaOAuthDeviceStart.safeParse({
					device_code: 'dev_1',
					verification_url: 'https://qoder.test/device/selectAccounts',
					user_code: 'AB12CD34',
					interval_seconds: 2,
					expires_in: 300,
					...testCase.patch
				}).success
			).toBe(false);
		});
	}

	it('reads a pending poll with no account fields', () => {
		const parsed = schemaOAuthDevicePoll.safeParse({ status: 'pending' });
		expect(parsed.success && parsed.data.endpoint_id).toBeUndefined();
		expect(parsed.success && parsed.data.created).toBe(false);
	});

	it('reads a connected poll with the endpoint it landed', () => {
		const parsed = schemaOAuthDevicePoll.safeParse({
			status: 'connected',
			endpoint_id: 'ep_1',
			token_hint: 'dt-…7c2f',
			created: true
		});
		expect(parsed.success && parsed.data.endpoint_id).toBe('ep_1');
		expect(parsed.success && oauthDeviceConnected(parsed.data)).toBe(true);
	});

	it('reads a verdict the panel does not know without failing, and does not treat it as a connect', () => {
		const parsed = schemaOAuthDevicePoll.safeParse({ status: 'slow_down' });
		expect(parsed.success).toBe(true);
		expect(parsed.success && oauthDeviceConnected(parsed.data)).toBe(false);
	});

	it('refuses a poll body that names no device code', () => {
		expect(schemaOAuthDevicePollBody.safeParse({}).success).toBe(false);
		expect(schemaOAuthDevicePollBody.safeParse({ device_code: 'dev_1' }).success).toBe(true);
	});
});
