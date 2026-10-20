// What a model test sweep says about itself, and which rows it offers (docs/SPEC-API/001-SPEC-API.md §7.4).
//
// The two response schemas are in `model-test.test.ts`. These are the readings a sweep hands back: the line
// a caller dials, the routable test that decides whether to offer it, and the summary sentence.

import { describe, expect, it } from 'vitest';
import {
	isChatRoutable,
	modelTestLine,
	modelTestSweepSummary,
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
