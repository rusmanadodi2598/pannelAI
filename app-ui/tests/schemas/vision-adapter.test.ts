// Vision adapter schema contract tests (docs/SPEC-API/001-SPEC-API.md §7.8, docs/SPEC-UI §6.4 tab 2).
//
// Two behaviours here are worth pinning rather than leaving to the template. The first is that `models` is
// always sent, even when empty: the API marks it required, so omitting it would be read as an error rather
// than as "no models". The second is the warning §6.4 requires, because "enabled with nothing selected" is
// a state that looks configured and adapts nothing.

import { describe, expect, it } from 'vitest';
import {
	VISION_MODELS_MAX,
	buildVisionAdapterBody,
	schemaVisionAdapter,
	schemaVisionAdapterForm,
	visionAdapterStatusText,
	visionAdapterToForm,
	visionAdapterWarning,
	visionModelRef
} from '$lib/schemas/vision-adapter';

function adapter(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		enabled: true,
		round_robin: false,
		models: ['openai/gpt-4o'],
		updated_at: '2026-09-18T00:00:00Z',
		...overrides
	};
}

describe('visionModelRef', () => {
	const ACCEPTED: { value: string; expected: string; why: string }[] = [
		{ value: 'openai/gpt-4o', expected: 'openai/gpt-4o', why: 'the provider/model form' },
		{ value: '  openai/gpt-4o  ', expected: 'openai/gpt-4o', why: 'surrounding whitespace' },
		{ value: 'a/b', expected: 'a/b', why: 'the shortest reference the API accepts' },
		{
			value: `${'p'.repeat(100)}/${'m'.repeat(99)}`,
			expected: `${'p'.repeat(100)}/${'m'.repeat(99)}`,
			why: 'a reference at 200 characters'
		}
	];

	for (const testCase of ACCEPTED) {
		it(`accepts ${testCase.why}`, () => {
			const result = visionModelRef.safeParse(testCase.value);

			expect(result.success).toBe(true);
			expect(result.success && result.data).toBe(testCase.expected);
		});
	}

	const REJECTED: { value: string; why: string }[] = [
		{ value: '', why: 'an empty reference' },
		{ value: 'gpt-4o', why: 'a bare model with no provider' },
		{ value: 'ab', why: 'a reference under 3 characters' },
		{ value: `${'p'.repeat(150)}/${'m'.repeat(100)}`, why: 'a reference over 200 characters' }
	];

	for (const testCase of REJECTED) {
		it(`rejects ${testCase.why}`, () => {
			expect(visionModelRef.safeParse(testCase.value).success).toBe(false);
		});
	}
});

describe('schemaVisionAdapter', () => {
	it('accepts a full configuration', () => {
		expect(schemaVisionAdapter.safeParse(adapter()).success).toBe(true);
	});

	it('accepts models as null, which is how Go marshals a nil slice, and normalizes to empty', () => {
		const result = schemaVisionAdapter.safeParse(adapter({ models: null }));

		expect(result.success).toBe(true);
		expect(result.success && result.data.models).toEqual([]);
	});

	it('accepts an omitted updated_at, because the field is marked omitempty', () => {
		const payload = adapter();
		delete payload.updated_at;

		expect(schemaVisionAdapter.safeParse(payload).success).toBe(true);
	});

	it('accepts a configuration that was never saved', () => {
		const result = schemaVisionAdapter.safeParse({
			enabled: false,
			round_robin: false,
			models: []
		});

		expect(result.success).toBe(true);
	});

	const INVALID: { payload: Record<string, unknown>; why: string }[] = [
		{ payload: adapter({ enabled: 'true' }), why: 'a boolean sent as a string' },
		{ payload: adapter({ round_robin: 1 }), why: 'a boolean sent as a number' },
		{ payload: adapter({ models: 'openai/gpt-4o' }), why: 'a model list sent as a string' },
		{ payload: adapter({ models: [7] }), why: 'a model that is not a string' }
	];

	for (const testCase of INVALID) {
		it(`rejects ${testCase.why}`, () => {
			expect(schemaVisionAdapter.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('schemaVisionAdapterForm', () => {
	it('accepts an empty model list, which is a real state the screen warns about', () => {
		const result = schemaVisionAdapterForm.safeParse({
			enabled: true,
			roundRobin: false,
			models: []
		});

		expect(result.success).toBe(true);
	});

	it('accepts a list at the 64 model limit', () => {
		const models = Array.from({ length: VISION_MODELS_MAX }, (_, index) => `p/m${index}`);

		expect(
			schemaVisionAdapterForm.safeParse({ enabled: true, roundRobin: true, models }).success
		).toBe(true);
	});

	it('rejects a list past the 64 model limit', () => {
		const models = Array.from({ length: VISION_MODELS_MAX + 1 }, (_, index) => `p/m${index}`);

		expect(
			schemaVisionAdapterForm.safeParse({ enabled: true, roundRobin: true, models }).success
		).toBe(false);
	});

	it('rejects a model that is not in the provider/model form', () => {
		const result = schemaVisionAdapterForm.safeParse({
			enabled: true,
			roundRobin: false,
			models: ['gpt-4o']
		});

		expect(result.success).toBe(false);
	});
});

describe('buildVisionAdapterBody', () => {
	it('always sends the model list, even when it is empty', () => {
		// The API marks `models` required, so an omitted list is a validation failure rather than "none".
		expect(buildVisionAdapterBody({ enabled: true, roundRobin: false, models: [] })).toEqual({
			enabled: true,
			round_robin: false,
			models: []
		});
	});

	it('renames the round robin field to the wire spelling', () => {
		const body = buildVisionAdapterBody({
			enabled: false,
			roundRobin: true,
			models: ['openai/gpt-4o']
		});

		expect(body).toEqual({ enabled: false, round_robin: true, models: ['openai/gpt-4o'] });
		expect(Object.keys(body).sort()).toEqual(['enabled', 'models', 'round_robin']);
	});

	it('copies the model list rather than sharing it with the form', () => {
		const form = { enabled: true, roundRobin: false, models: ['openai/gpt-4o'] };
		const body = buildVisionAdapterBody(form);

		expect(body.models).toEqual(form.models);
		expect(body.models).not.toBe(form.models);
	});
});

describe('visionAdapterToForm', () => {
	it('renames the round robin field to the form spelling', () => {
		const parsed = schemaVisionAdapter.parse(adapter({ round_robin: true }));
		const result = visionAdapterToForm(parsed);

		expect(result).toEqual({ enabled: true, roundRobin: true, models: ['openai/gpt-4o'] });
	});

	it('round-trips an empty configuration', () => {
		const parsed = schemaVisionAdapter.parse({ enabled: false, round_robin: false, models: [] });

		expect(visionAdapterToForm(parsed)).toEqual({
			enabled: false,
			roundRobin: false,
			models: []
		});
	});
});

describe('visionAdapterWarning', () => {
	const CASES: { enabled: boolean; models: string[]; expected: string | null; why: string }[] = [
		{ enabled: false, models: [], expected: null, why: 'the adapter is off, so nothing is wrong' },
		{
			enabled: false,
			models: ['openai/gpt-4o'],
			expected: null,
			why: 'off with a model selected is still off'
		},
		{
			enabled: true,
			models: ['openai/gpt-4o'],
			expected: null,
			why: 'on with a model selected is the configured state'
		},
		{
			enabled: true,
			models: [],
			expected:
				'The adapter is enabled but no model is selected, so image requests are not adapted until you choose at least one.',
			why: 'on with nothing selected adapts nothing, which §6.4 requires the screen to say'
		}
	];

	for (const testCase of CASES) {
		it(`returns ${testCase.expected === null ? 'no warning' : 'a warning'} when ${testCase.why}`, () => {
			expect(visionAdapterWarning(testCase)).toBe(testCase.expected);
		});
	}
});

describe('visionAdapterStatusText', () => {
	const CASES: {
		adapter: { enabled: boolean; roundRobin: boolean; models: string[] };
		expected: string;
		why: string;
	}[] = [
		{
			adapter: { enabled: false, roundRobin: false, models: [] },
			expected: 'Off',
			why: 'the adapter is off'
		},
		{
			adapter: { enabled: false, roundRobin: true, models: ['openai/gpt-4o'] },
			expected: 'Off',
			why: 'off is off regardless of the rotation setting'
		},
		{
			adapter: { enabled: true, roundRobin: false, models: [] },
			expected: 'On, with no model selected',
			why: 'on with nothing selected'
		},
		{
			adapter: { enabled: true, roundRobin: false, models: ['openai/gpt-4o'] },
			expected: 'On, 1 model in order',
			why: 'on with one model, singular, in order'
		},
		{
			adapter: { enabled: true, roundRobin: false, models: ['a/b', 'c/d'] },
			expected: 'On, 2 models in order',
			why: 'on with several models, plural, in order'
		},
		{
			adapter: { enabled: true, roundRobin: true, models: ['a/b', 'c/d', 'e/f'] },
			expected: 'On, 3 models round robin',
			why: 'on with several models rotating'
		}
	];

	for (const testCase of CASES) {
		it(`says "${testCase.expected}" when ${testCase.why}`, () => {
			expect(visionAdapterStatusText(testCase.adapter)).toBe(testCase.expected);
		});
	}
});
