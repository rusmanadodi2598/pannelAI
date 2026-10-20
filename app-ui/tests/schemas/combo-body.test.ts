// The body a combo save writes (docs/SPEC-API/001-SPEC-API.md §7.6).
//
// The form it reads is in `combo-form.test.ts`. This is the one place the panel decides what the API gets, so
// it is pinned on its own: a field dropped here is a setting the gateway never hears about.

import { describe, expect, it } from 'vitest';
import { buildComboBody, type ComboForm } from '$lib/schemas/combo-form';
import { form } from '../support/combo-fixtures';
describe('buildComboBody', () => {
	const CASES: {
		input: Partial<ComboForm>;
		expected: Record<string, unknown>;
		why: string;
	}[] = [
		{
			input: { strategy: 'fallback', stickyLimit: 1, judgeModel: '' },
			expected: {
				name: 'daily',
				strategy: 'fallback',
				models: [{ ref: 'openai/gpt-4o', priority: 0 }]
			},
			why: 'fallback sends neither optional field'
		},
		{
			input: { strategy: 'round_robin', stickyLimit: 5 },
			expected: {
				name: 'daily',
				strategy: 'round_robin',
				models: [{ ref: 'openai/gpt-4o', priority: 0 }],
				sticky_limit: 5
			},
			why: 'round_robin sends the sticky limit and no judge'
		},
		{
			input: { strategy: 'fusion', judgeModel: 'openai/gpt-4o-mini' },
			expected: {
				name: 'daily',
				strategy: 'fusion',
				models: [{ ref: 'openai/gpt-4o', priority: 0 }],
				judge_model: 'openai/gpt-4o-mini'
			},
			why: 'fusion sends the judge model and no sticky limit'
		},
		{
			input: { strategy: 'fallback', judgeModel: 'openai/gpt-4o-mini' },
			expected: {
				name: 'daily',
				strategy: 'fallback',
				models: [{ ref: 'openai/gpt-4o', priority: 0 }]
			},
			why: 'a leftover judge model is dropped rather than sent, which is what the API requires'
		},
		{
			input: { strategy: 'fusion', stickyLimit: 9, judgeModel: 'j/m' },
			expected: {
				name: 'daily',
				strategy: 'fusion',
				models: [{ ref: 'openai/gpt-4o', priority: 0 }],
				judge_model: 'j/m'
			},
			why: 'a leftover sticky limit is dropped for fusion'
		}
	];

	for (const testCase of CASES) {
		it(testCase.why, () => {
			expect(buildComboBody(form(testCase.input))).toEqual(testCase.expected);
		});
	}

	it('preserves the model order it was given, because the order is the fallback chain', () => {
		const body = buildComboBody(
			form({
				models: [
					{ ref: 'third/m', priority: 20 },
					{ ref: 'first/m', priority: 0 },
					{ ref: 'second/m', priority: 10 }
				]
			})
		);

		expect(body.models).toEqual([
			{ ref: 'third/m', priority: 20 },
			{ ref: 'first/m', priority: 0 },
			{ ref: 'second/m', priority: 10 }
		]);
	});

	it('always sends the name, the strategy, and the model list', () => {
		const body = buildComboBody(form({ strategy: 'round_robin', stickyLimit: 2 }));

		expect(Object.keys(body).sort()).toEqual(['models', 'name', 'sticky_limit', 'strategy']);
	});
});
