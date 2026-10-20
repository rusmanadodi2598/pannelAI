// What an OAuth read accepts (docs/SPEC-API/001-SPEC-API.md §7.4, docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The flow helpers and the callback parse are in `oauth.test.ts`. This is the larger half: the accounts and
// endpoint rows the provider screens read, pinned field by field because a row the panel cannot name still
// has to render.

import { describe, expect, it } from 'vitest';
import {
	oauthDeviceConnected,
	schemaOAuthDevicePoll,
	schemaOAuthDevicePollBody,
	schemaOAuthDeviceStart,
	schemaOAuthRefresh,
	schemaOAuthRefreshBody,
	schemaOAuthStart,
	schemaOAuthStatus
} from '$lib/schemas/oauth';
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

	it('treats an absent skip list as nobody being passed over', () => {
		const parsed = schemaOAuthRefresh.safeParse({ refreshed: 1, endpoint_ids: ['ep_1'] });
		expect(parsed.success && parsed.data.skipped).toEqual([]);
	});

	it('keeps the account a sweep skipped beside the reason it stopped there', () => {
		const parsed = schemaOAuthRefresh.safeParse({
			refreshed: 1,
			endpoint_ids: ['ep_clean'],
			skipped: [{ endpoint_id: 'ep_race', reason: 'the endpoint changed during this refresh' }]
		});
		expect(parsed.success && parsed.data.skipped).toEqual([
			{ endpoint_id: 'ep_race', reason: 'the endpoint changed during this refresh' }
		]);
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
