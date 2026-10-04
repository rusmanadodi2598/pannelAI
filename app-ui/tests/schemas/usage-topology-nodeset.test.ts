// Which entities get a node (src/lib/schemas/usage-topology-nodeset.ts, draft 012 F3 and draft 043 F1).
//
// The provider rows are the difference between a provider the gateway knows and one it could route anything
// through. The combo rows are the difference between a combo the operator defined and one the band can still
// draw a readable label for — a cap the panel has to state rather than quietly apply.

import { describe, expect, it } from 'vitest';
import type { Combo } from '$lib/schemas/combo';
import type { Provider } from '$lib/schemas/provider';
import {
	COMBO_MAX_NODES,
	configuredCombos,
	configuredProviders,
	providerNameMap
} from '$lib/schemas/usage-topology-nodeset';
import { forEachCase } from '../support/tables';

function provider(overrides: Partial<Provider> & { id: string }): Provider {
	return {
		name: overrides.id,
		category: 'apikey',
		auth_type: 'api_key',
		auth_modes: [],
		auth_hint: null,
		has_oauth: false,
		no_auth: false,
		routability: 'routable',
		endpoint_count: 0,
		status_summary: { total: 0, active: 0, disabled: 0, error: 0, rate_limited: 0 },
		...overrides
	};
}

function combo(name: string): Combo {
	return {
		id: `cmb_${name}`,
		name,
		strategy: 'fallback',
		sticky_limit: 1,
		judge_model: '',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		created_at: null,
		updated_at: null
	};
}

describe('configuredProviders', () => {
	it('keeps a provider with a stored endpoint', () => {
		const kept = configuredProviders([provider({ id: 'openai', endpoint_count: 2 })]);

		expect(kept).toEqual([{ id: 'openai', name: 'openai' }]);
	});

	it('drops a provider nothing has been configured for', () => {
		expect(configuredProviders([provider({ id: 'openai' })])).toEqual([]);
	});

	it('keeps a provider that needs no credential, because its endpoint count stays at zero', () => {
		const kept = configuredProviders([provider({ id: 'opencode', no_auth: true })]);

		expect(kept).toEqual([{ id: 'opencode', name: 'opencode' }]);
	});

	it('sorts by id so the drawing is the same drawing on every read', () => {
		const kept = configuredProviders([
			provider({ id: 'openai', endpoint_count: 1 }),
			provider({ id: 'anthropic', endpoint_count: 1 }),
			provider({ id: 'zai', no_auth: true })
		]);

		expect(kept.map((entry) => entry.id)).toEqual(['anthropic', 'openai', 'zai']);
	});
});

describe('configuredCombos', () => {
	it('uses the name as the node identity, because that is the string the client addresses', () => {
		expect(configuredCombos([combo('pro-tier')])).toEqual([{ id: 'pro-tier', name: 'pro-tier' }]);
	});

	it('keeps every combo the operator defined, because a combo needs no endpoint to route', () => {
		const kept = configuredCombos([combo('fast'), combo('pro-tier')]);

		expect(kept.map((entry) => entry.id)).toEqual(['fast', 'pro-tier']);
	});

	it('sorts by name so the band is the same band on every read', () => {
		const kept = configuredCombos([combo('zeta'), combo('alpha'), combo('mid')]);

		expect(kept.map((entry) => entry.id)).toEqual(['alpha', 'mid', 'zeta']);
	});

	it('stops the band where the labels stop being readable', () => {
		const many = Array.from({ length: COMBO_MAX_NODES + 5 }, (_unused, index) =>
			combo(`combo-${String(index).padStart(2, '0')}`)
		);

		expect(configuredCombos(many)).toHaveLength(COMBO_MAX_NODES);
	});

	const capCases = [
		{ name: 'no combo', count: 0, want: 0 },
		{ name: 'one combo', count: 1, want: 1 },
		{ name: 'a band exactly at the cap', count: COMBO_MAX_NODES, want: COMBO_MAX_NODES },
		{ name: 'a band past the cap', count: COMBO_MAX_NODES + 3, want: COMBO_MAX_NODES }
	];

	forEachCase(capCases, (testCase) => {
		const list = Array.from({ length: testCase.count }, (_unused, index) =>
			combo(`combo-${String(index).padStart(2, '0')}`)
		);

		expect(configuredCombos(list)).toHaveLength(testCase.want);
	});
});

describe('providerNameMap', () => {
	it('resolves an id without case, and leaves an unknown one out', () => {
		const names = providerNameMap([{ id: 'OpenAI', name: 'OpenAI' }]);

		expect(names.get('openai')).toBe('OpenAI');
		expect(names.has('anthropic')).toBe(false);
	});
});
