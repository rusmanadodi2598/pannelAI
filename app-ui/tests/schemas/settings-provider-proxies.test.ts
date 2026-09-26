// The per-provider proxy binding schemas (docs/PORT/009-PORT-PROVIDER-PROXY.md D1-D3, D9;
// docs/SPEC-API/001-SPEC-API.md §7.14).
//
// Three shapes with three jobs. The read schema parses one entry of `network.provider_proxies` as
// `GET /settings` answers it, and both its fields stay optional because an absent one inherits the
// global setting: the panel renders what is stored rather than a filled-in copy, so an entry that
// only pins a pool must not read as if it had also chosen a strategy. The patch schema is the entry
// as the card sends it, and it is strict, so a field the card does not know is a failure rather than
// a silent drop. The wrapper is a separate patch shape from the network tab's for the same reason
// `schemaProviderStrategiesPatch` is separate from the routing tab's: the two own different keys.
//
// `__none__` is the reference's own sentinel (NoAuthProxyCard.js), and the panel names it once so the
// select, the copy, and the body cannot disagree about how "None" is spelled.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	PROXY_POOL_NONE,
	schemaNetworkSettings,
	schemaProviderProxiesPatch,
	schemaProviderProxy,
	schemaProviderProxyPatch
} from '$lib/schemas/settings';

describe('schemaProviderProxy', () => {
	const cases = [
		{
			name: 'accepts a pinned pool with its own strategy',
			input: { pool_id: 'prx_01HZZ9K2', strategy: 'fallback' },
			ok: true
		},
		{ name: 'accepts the none sentinel', input: { pool_id: PROXY_POOL_NONE }, ok: true },
		{
			name: 'accepts a strategy-only entry, which keeps the global pool',
			input: { strategy: 'round_robin' },
			ok: true
		},
		{
			name: 'rejects a strategy the engine does not run',
			input: { pool_id: 'prx_01HZZ9K2', strategy: 'random' },
			ok: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaProviderProxy.safeParse(testCase.input).success, testCase.name).toBe(testCase.ok);
	});
});

describe('schemaProviderProxyPatch', () => {
	const cases = [
		{ name: 'accepts a pinned pool alone', input: { pool_id: 'prx_01HZZ9K2' }, ok: true },
		{ name: 'accepts a strategy alone', input: { strategy: 'fallback' }, ok: true },
		{
			name: 'accepts both, which is a pin with its own order',
			input: { pool_id: PROXY_POOL_NONE, strategy: 'round_robin' },
			ok: true
		},
		{
			name: 'accepts the longest id the API stores',
			input: { pool_id: 'p'.repeat(64) },
			ok: true
		},
		{
			name: 'rejects an id longer than the API stores',
			input: { pool_id: 'p'.repeat(65) },
			ok: false
		},
		{
			name: 'rejects a strategy the engine does not run',
			input: { strategy: 'random' },
			ok: false
		},
		{ name: 'rejects an unknown key', input: { pool: 'prx_01HZZ9K2' }, ok: false }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaProviderProxyPatch.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});
});

describe('schemaProviderProxiesPatch', () => {
	it('accepts a whole map, because the map is one settings value', () => {
		const parsed = schemaProviderProxiesPatch.safeParse({
			network: {
				provider_proxies: {
					openai: { pool_id: 'prx_01HZZ9K2', strategy: 'round_robin' },
					anthropic: { pool_id: PROXY_POOL_NONE }
				}
			}
		});

		expect(parsed.success).toBe(true);
	});

	it('accepts an empty map, which is how the last binding is removed', () => {
		expect(
			schemaProviderProxiesPatch.safeParse({ network: { provider_proxies: {} } }).success
		).toBe(true);
	});

	const cases = [
		{ name: 'rejects a body with no network group', input: {}, ok: false },
		{ name: 'rejects a network group with no map', input: { network: {} }, ok: false },
		{
			name: 'rejects a network key beside the map, because the card owns the map alone',
			input: { network: { provider_proxies: {}, outbound_proxy_enabled: true } },
			ok: false
		},
		{
			name: 'rejects an unknown top-level group',
			input: { routing: { provider_proxies: {} } },
			ok: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaProviderProxiesPatch.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});
});

describe('schemaNetworkSettings', () => {
	const base = {
		outbound_proxy_enabled: false,
		outbound_proxy_url: '',
		outbound_no_proxy: '',
		outbound_proxy_strategy: 'fallback'
	};

	it('requires the binding map, which the API always sends', () => {
		// The contract lists `provider_proxies` as required on the response, so a document without it
		// is drift rather than an older answer, and the read schema says so.
		expect(schemaNetworkSettings.safeParse(base).success).toBe(false);
	});

	it('reads an empty map as an object, not as null', () => {
		const parsed = schemaNetworkSettings.safeParse({ ...base, provider_proxies: {} });

		expect(parsed.success).toBe(true);
		expect(parsed.success && parsed.data.provider_proxies).toEqual({});
	});

	it('reads a stored binding without filling in the field it left out', () => {
		const parsed = schemaNetworkSettings.safeParse({
			...base,
			provider_proxies: { openai: { pool_id: 'prx_01HZZ9K2' } }
		});

		expect(parsed.success && parsed.data.provider_proxies.openai).toEqual({
			pool_id: 'prx_01HZZ9K2'
		});
	});
});
