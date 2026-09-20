// Tests for the combo test contracts (docs/SPEC-API/001-SPEC-API.md §7.7, docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// Each group is a table of input variations, per docs/RULLES/TDD.md §2.5. Three of the rules come from the
// route's own contract: a failed probe is a result rather than an error, the identity fields are `omitempty`
// so a probe that never resolved carries none of them, and `role` names which part of the combo a reference
// plays. The summary and the identity are the panel's own sentences, so they are tested as text.

import { describe, expect, it } from 'vitest';
import {
	comboProbeIdentity,
	comboProbeRoleLabel,
	comboProbeSummary,
	schemaComboProbeResult,
	schemaComboTest,
	type ComboProbeResult
} from '$lib/schemas/combo-test';

function probe(overrides: Partial<ComboProbeResult> = {}): ComboProbeResult {
	return {
		ref: 'openai/gpt-4o',
		role: 'model',
		ok: true,
		provider_id: 'openai',
		model_id: 'gpt-4o',
		endpoint_id: 'ep_01',
		latency_ms: 120,
		error_code: '',
		error: '',
		...overrides
	};
}

describe('schemaComboTest', () => {
	const cases = [
		{
			name: 'a full answer',
			payload: {
				combo_id: 'cmb_01',
				combo: 'daily',
				strategy: 'fallback',
				results: [
					{
						ref: 'openai/gpt-4o',
						role: 'model',
						ok: true,
						provider_id: 'openai',
						model_id: 'gpt-4o',
						endpoint_id: 'ep_01',
						latency_ms: 120
					}
				]
			},
			want: 1
		},
		{
			// `omitempty` on the wire: a probe that failed before it resolved a target carries none of these.
			name: 'a failed probe with no identity and no error text',
			payload: {
				combo_id: 'cmb_01',
				combo: 'daily',
				strategy: 'fusion',
				results: [{ ref: 'smart', role: 'judge', ok: false, latency_ms: 0 }]
			},
			want: 1
		},
		{
			name: 'an absent result list',
			payload: { combo_id: 'cmb_01', combo: 'daily', strategy: 'fallback' },
			want: 0
		},
		{
			name: 'an empty result list',
			payload: { combo_id: 'cmb_01', combo: 'daily', strategy: 'fallback', results: [] },
			want: 0
		},
		{
			name: 'a role and a strategy the panel does not know',
			payload: {
				combo_id: 'cmb_01',
				combo: 'daily',
				strategy: 'cascade',
				results: [{ ref: 'a/b', role: 'shadow', ok: true, latency_ms: 5 }]
			},
			want: 1
		}
	];

	for (const testCase of cases) {
		it(`reads ${testCase.name}`, () => {
			const parsed = schemaComboTest.safeParse(testCase.payload);
			expect(parsed.success).toBe(true);
			expect(parsed.success && parsed.data.results.length).toBe(testCase.want);
		});
	}

	it('reads an absent identity field as the empty string rather than as a missing key', () => {
		const parsed = schemaComboTest.safeParse({
			combo_id: 'cmb_01',
			combo: 'daily',
			strategy: 'fallback',
			results: [{ ref: 'a/b', role: 'model', ok: true, latency_ms: 5 }]
		});

		expect(parsed.success && parsed.data.results[0]).toEqual({
			ref: 'a/b',
			role: 'model',
			ok: true,
			provider_id: '',
			model_id: '',
			endpoint_id: '',
			latency_ms: 5,
			error_code: '',
			error: ''
		});
	});
});

describe('schemaComboProbeResult', () => {
	const cases = [
		{ name: 'a missing ok', payload: { ref: 'a/b', role: 'model', latency_ms: 1 } },
		{
			name: 'a negative latency',
			payload: { ref: 'a/b', role: 'model', ok: true, latency_ms: -1 }
		},
		{
			name: 'a fractional latency',
			payload: { ref: 'a/b', role: 'model', ok: true, latency_ms: 1.5 }
		},
		{ name: 'an empty ref', payload: { ref: '', role: 'model', ok: true, latency_ms: 1 } }
	];

	for (const testCase of cases) {
		it(`refuses ${testCase.name}`, () => {
			expect(schemaComboProbeResult.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('comboProbeSummary', () => {
	const cases = [
		{ name: 'no references at all', results: [], contains: 'nothing to probe' },
		{
			name: 'the only reference answering',
			results: [probe()],
			contains: 'only reference answered'
		},
		{
			name: 'the only reference failing',
			results: [probe({ ok: false })],
			contains: 'only reference did not answer'
		},
		{
			name: 'every reference answering',
			results: [probe(), probe()],
			contains: 'All 2 references'
		},
		{
			name: 'no reference answering',
			results: [probe({ ok: false }), probe({ ok: false })],
			contains: 'None of the 2 references'
		},
		{
			name: 'some references answering',
			results: [probe(), probe({ ok: false }), probe()],
			contains: '2 of 3 references'
		}
	];

	for (const testCase of cases) {
		it(`counts ${testCase.name}`, () => {
			expect(comboProbeSummary(testCase.results)).toContain(testCase.contains);
		});
	}
});

describe('comboProbeRoleLabel', () => {
	const cases = [
		{ name: 'model', role: 'model', want: 'Model' },
		{ name: 'judge', role: 'judge', want: 'Judge' },
		{ name: 'a role the panel does not know', role: 'shadow', want: 'shadow' }
	];

	for (const testCase of cases) {
		it(`labels ${testCase.name}`, () => {
			expect(comboProbeRoleLabel(testCase.role)).toBe(testCase.want);
		});
	}
});

describe('comboProbeIdentity', () => {
	const cases = [
		{ name: 'a resolved pair', result: probe(), want: 'openai/gpt-4o' },
		{
			name: 'a provider with no model',
			result: probe({ model_id: '' }),
			want: 'openai'
		},
		{
			name: 'a model with no provider',
			result: probe({ provider_id: '' }),
			want: 'gpt-4o'
		},
		{
			name: 'nothing resolved at all',
			result: probe({ provider_id: '', model_id: '' }),
			want: 'No identity was reported.'
		}
	];

	for (const testCase of cases) {
		it(`reads ${testCase.name}`, () => {
			expect(comboProbeIdentity(testCase.result)).toBe(testCase.want);
		});
	}
});
