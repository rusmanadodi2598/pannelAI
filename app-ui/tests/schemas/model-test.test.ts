// Tests for the provider model test contracts (docs/SPEC-API/001-SPEC-API.md §7.4, draft 017 §4.10).
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
	isChatRoutable,
	modelTestLine,
	modelTestRowKey,
	modelTestSweepSummary,
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

describe('isChatRoutable', () => {
	const cases = [
		{ kind: undefined, want: true, name: 'a model that declares no kind' },
		{ kind: '', want: true, name: 'a model with an empty kind' },
		{ kind: 'llm', want: true, name: 'an llm' },
		{ kind: 'chat', want: true, name: 'a chat model' },
		{ kind: 'embedding', want: false, name: 'an embedding model' },
		{ kind: 'image', want: false, name: 'an image model' },
		{ kind: 'stt', want: false, name: 'a transcription model' }
	];

	for (const testCase of cases) {
		it(testCase.name, () => {
			expect(isChatRoutable(testCase.kind)).toBe(testCase.want);
		});
	}
});

describe('modelTestLine', () => {
	const cases = [
		{ name: 'answered', row: result(), want: 'Answered in 214 ms' },
		{
			name: 'answered without an endpoint',
			row: result({ endpoint_id: '' }),
			want: 'Answered in 214 ms'
		},
		{
			name: 'refused with a code',
			row: result({
				ok: false,
				latency_ms: 8,
				error_code: 'RATE_LIMITED',
				error: 'quota spent',
				status: 429
			}),
			want: 'Failed in 8 ms'
		},
		{
			name: 'timed out',
			row: result({
				ok: false,
				latency_ms: 0,
				error_code: 'MODEL_TEST_TIMEOUT',
				error: 'no answer'
			}),
			want: 'Failed'
		}
	];

	for (const testCase of cases) {
		it(testCase.name, () => {
			expect(modelTestLine(testCase.row)).toBe(testCase.want);
		});
	}
});

describe('modelTestSweepSummary', () => {
	function sweep(overrides: Partial<Parameters<typeof modelTestSweepSummary>[0]> = {}) {
		return {
			provider_id: 'openai',
			source: 'registry',
			warning: '',
			tested: 3,
			total: 3,
			stopped: '',
			results: [
				result(),
				result({ model_id: 'b', ok: false, error_code: 'RATE_LIMITED' }),
				result({ model_id: 'c', ok: true })
			],
			...overrides
		};
	}

	const cases = [
		{ name: 'mixed results', row: sweep(), want: 'Tested 3 models: 2 answered, 1 failed.' },
		{
			name: 'every model answered',
			row: sweep({ tested: 2, total: 2, results: [result(), result({ model_id: 'b' })] }),
			want: 'Tested 2 models: all answered.'
		},
		{
			name: 'no model answered',
			row: sweep({
				tested: 2,
				total: 2,
				results: [result({ ok: false }), result({ model_id: 'b', ok: false })]
			}),
			want: 'Tested 2 models: none answered.'
		},
		{
			name: 'one model only',
			row: sweep({ tested: 1, total: 1, results: [result()] }),
			want: 'Tested 1 model: all answered.'
		},
		{
			name: 'a truncated sweep',
			row: sweep({
				tested: 2,
				total: 9,
				results: [result(), result({ model_id: 'b', ok: false, error_code: 'RATE_LIMITED' })]
			}),
			want: 'Tested 2 of 9 models: 1 answered, 1 failed.'
		},
		{
			name: 'a sweep cut short by its budget',
			row: sweep({ tested: 1, total: 9, stopped: 'deadline', results: [result()] }),
			want: 'Tested 1 of 9 models: all answered. The sweep ran out of time before every model.'
		},
		{
			name: 'nothing to test',
			row: sweep({ tested: 0, total: 0, results: [] }),
			want: 'This provider offers no chat model to test.'
		}
	];

	for (const testCase of cases) {
		it(testCase.name, () => {
			expect(modelTestSweepSummary(testCase.row)).toBe(testCase.want);
		});
	}

	it('appends the warning the API reported in its own words', () => {
		const warning = 'The provider did not answer, so this list is the saved one.';
		const line = modelTestSweepSummary(sweep({ warning }));
		expect(line).toBe(`Tested 3 models: 2 answered, 1 failed. ${warning}`);
	});
});
