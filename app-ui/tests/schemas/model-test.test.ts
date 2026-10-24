// Tests for the provider model test contracts (docs/SPEC-API/001-SPEC-API.md §7.4).
//
// The route inverts the usual rule: a model that refused to answer is a RESULT, and only a request that
// was not a question is an error. So the schema's job is to keep a failure readable without inventing
// anything, and to keep "the sweep was cut short" distinct from "every model failed". Each group is a table
// of input variations per docs/RULLES/TDD.md §2.5.
//
// `status` is read as 0 when absent because the wire marks it `omitempty`: a healthy probe never saw an
// HTTP code the gateway could report, and defaulting it to 200 would put a figure on screen that no
// response carried.

import { describe, expect, it } from 'vitest';
import {
	modelTestRowKey,
	schemaModelTestResult,
	schemaModelTestSweep,
	type ModelTestResult
} from '$lib/schemas/model-test';

function result(overrides: Partial<ModelTestResult> = {}): ModelTestResult {
	return {
		model_id: 'gpt-4o',
		name: 'GPT-4o',
		ok: true,
		latency_ms: 214,
		endpoint_id: 'ep_01',
		status: 0,
		error_code: '',
		error: '',
		...overrides
	};
}

describe('schemaModelTestResult', () => {
	const cases = [
		{
			name: 'a healthy probe',
			payload: {
				model_id: 'gpt-4o',
				name: 'GPT-4o',
				ok: true,
				latency_ms: 214,
				endpoint_id: 'ep_01'
			},
			want: result()
		},
		{
			name: 'a refused probe',
			payload: {
				model_id: 'gpt-4o',
				ok: false,
				latency_ms: 8,
				status: 429,
				error_code: 'RATE_LIMITED',
				error: 'quota spent'
			},
			want: result({
				name: '',
				ok: false,
				latency_ms: 8,
				endpoint_id: '',
				status: 429,
				error_code: 'RATE_LIMITED',
				error: 'quota spent'
			})
		},
		{
			name: 'a probe that never resolved a target',
			payload: { model_id: 'x', ok: false, latency_ms: 0, error_code: 'NO_PROVIDER_AVAILABLE' },
			want: result({
				model_id: 'x',
				name: '',
				ok: false,
				latency_ms: 0,
				endpoint_id: '',
				error_code: 'NO_PROVIDER_AVAILABLE'
			})
		}
	];

	for (const testCase of cases) {
		it(testCase.name, () => {
			expect(schemaModelTestResult.parse(testCase.payload)).toEqual(testCase.want);
		});
	}

	it('refuses a row with no model id', () => {
		expect(() => schemaModelTestResult.parse({ ok: true, latency_ms: 1 })).toThrow();
	});

	it('refuses a negative latency', () => {
		expect(() =>
			schemaModelTestResult.parse({ model_id: 'x', ok: true, latency_ms: -1 })
		).toThrow();
	});
});

describe('schemaModelTestSweep', () => {
	it('reads a truncated sweep', () => {
		const payload = {
			provider_id: 'openai',
			source: 'registry',
			tested: 2,
			total: 41,
			results: [{ model_id: 'gpt-4o', ok: true, latency_ms: 10 }]
		};
		expect(schemaModelTestSweep.parse(payload)).toEqual({
			provider_id: 'openai',
			source: 'registry',
			warning: '',
			tested: 2,
			total: 41,
			stopped: '',
			results: [result({ name: '', latency_ms: 10, endpoint_id: '' })]
		});
	});

	it('reads a sweep that ran out of budget with a stale-list warning', () => {
		const parsed = schemaModelTestSweep.parse({
			provider_id: 'node-1',
			source: 'registry',
			warning: 'The provider did not answer, so this list is the saved one.',
			tested: 1,
			total: 9,
			stopped: 'deadline',
			results: []
		});
		expect(parsed.stopped).toBe('deadline');
		expect(parsed.warning).toBe('The provider did not answer, so this list is the saved one.');
		expect(parsed.results).toEqual([]);
	});

	it('refuses an answer with no source', () => {
		expect(() =>
			schemaModelTestSweep.parse({ provider_id: 'p', tested: 0, total: 0, results: [] })
		).toThrow();
	});
});

describe('modelTestRowKey', () => {
	it('names the row the way a client addresses the model', () => {
		expect(modelTestRowKey('openai', 'gpt-4o')).toBe('openai/gpt-4o');
	});
});
