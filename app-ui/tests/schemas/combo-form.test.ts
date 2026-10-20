// The combo form: what it accepts, what a row fills into it, and what the summary line says
// (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The body the form writes is in `combo-body.test.ts`, and the row schemas are in `combo.test.ts`. This is the
// editor's own contract, including the model it shows for a row whose ref no longer resolves.

import { describe, expect, it } from 'vitest';
import { COMBO_STRATEGIES, comboModelSummary, schemaCombo } from '$lib/schemas/combo';
import { combo, form } from '../support/combo-fixtures';
import { comboToForm, schemaComboForm } from '$lib/schemas/combo-form';
describe('schemaComboForm', () => {
	it('accepts a fallback combo', () => {
		expect(schemaComboForm.safeParse(form()).success).toBe(true);
	});

	it('accepts a round_robin combo with a sticky limit of 1', () => {
		expect(
			schemaComboForm.safeParse(form({ strategy: 'round_robin', stickyLimit: 1 })).success
		).toBe(true);
	});

	it('accepts a fusion combo with a judge model', () => {
		const result = schemaComboForm.safeParse(
			form({ strategy: 'fusion', judgeModel: 'openai/gpt-4o' })
		);

		expect(result.success).toBe(true);
	});

	it('accepts a fallback combo that still carries a judge model, because the body drops it', () => {
		// The field is hidden for fallback, so a value can only be a leftover from another strategy. It is
		// not an error: `buildComboBody` omits it, which is what the API requires.
		expect(schemaComboForm.safeParse(form({ judgeModel: 'openai/gpt-4o' })).success).toBe(true);
	});

	it('rejects a fusion combo with no judge model', () => {
		const result = schemaComboForm.safeParse(form({ strategy: 'fusion', judgeModel: '' }));

		expect(result.success).toBe(false);
		expect(result.success || result.error.issues.some((i) => i.path[0] === 'judgeModel')).toBe(
			true
		);
	});

	it('rejects a round_robin combo with a sticky limit of zero', () => {
		const result = schemaComboForm.safeParse(form({ strategy: 'round_robin', stickyLimit: 0 }));

		expect(result.success).toBe(false);
		expect(result.success || result.error.issues.some((i) => i.path[0] === 'stickyLimit')).toBe(
			true
		);
	});

	it('rejects a combo with no models', () => {
		const result = schemaComboForm.safeParse(form({ models: [] }));

		expect(result.success).toBe(false);
		expect(result.success || result.error.issues.some((i) => i.path[0] === 'models')).toBe(true);
	});

	it('rejects a combo listing the same reference twice', () => {
		const result = schemaComboForm.safeParse(
			form({
				models: [
					{ ref: 'a/b', priority: 0 },
					{ ref: 'a/b', priority: 1 }
				]
			})
		);

		expect(result.success).toBe(false);
		expect(result.success || result.error.issues.some((i) => i.message.includes('a/b'))).toBe(true);
	});

	it('rejects a combo over the 64 model limit', () => {
		const models = Array.from({ length: 65 }, (_, index) => ({
			ref: `p/m${index}`,
			priority: index
		}));

		expect(schemaComboForm.safeParse(form({ models })).success).toBe(false);
	});

	it('rejects a fractional sticky limit', () => {
		expect(schemaComboForm.safeParse(form({ stickyLimit: 1.5 })).success).toBe(false);
	});
});

describe('comboToForm', () => {
	it('keeps a round_robin combo sticky limit', () => {
		const parsed = schemaCombo.parse(combo({ strategy: 'round_robin', sticky_limit: 4 }));
		const result = comboToForm(parsed);

		expect(result.strategy).toBe('round_robin');
		expect(result.stickyLimit).toBe(4);
	});

	it('gives a fallback combo a usable sticky limit, because the field can become visible again', () => {
		// The API normalizes a fallback combo's sticky limit to 1, so seeding the form with it means the
		// operator who switches to round_robin sees a value that already satisfies the rule.
		const parsed = schemaCombo.parse(combo({ strategy: 'fallback', sticky_limit: 1 }));

		expect(comboToForm(parsed).stickyLimit).toBe(1);
	});

	it('carries a fusion combo judge model into the form', () => {
		const parsed = schemaCombo.parse(
			combo({ strategy: 'fusion', judge_model: 'openai/gpt-4o-mini', sticky_limit: 0 })
		);

		expect(comboToForm(parsed).judgeModel).toBe('openai/gpt-4o-mini');
	});

	it('falls back to the first strategy when the API reports one the panel does not offer', () => {
		// The form's select can only render a known strategy, so an unknown one is normalized rather than
		// left to render a blank option. The combo's own value is untouched on the server.
		const parsed = schemaCombo.parse(combo({ strategy: 'weighted' }));
		const result = comboToForm(parsed);

		expect(result.strategy).toBe('fallback');
		expect(COMBO_STRATEGIES).toContain(result.strategy);
	});

	it('copies the model list rather than sharing it with the parsed combo', () => {
		const parsed = schemaCombo.parse(combo());
		const result = comboToForm(parsed);

		expect(result.models).toEqual(parsed.models);
		expect(result.models).not.toBe(parsed.models);
	});
});

describe('comboModelSummary', () => {
	const CASES: { refs: string[]; expected: string; why: string }[] = [
		{
			refs: [],
			expected: '0 models',
			why: 'no models, which the API refuses but the panel still renders'
		},
		{ refs: ['a/b'], expected: '1 model', why: 'a single model, singular' },
		{ refs: ['a/b', 'c/d'], expected: '2 models', why: 'two models, plural' },
		{
			refs: Array.from({ length: 40 }, (_, i) => `p/m${i}`),
			expected: '40 models',
			why: 'a large list'
		}
	];

	for (const testCase of CASES) {
		it(`says "${testCase.expected}" for ${testCase.why}`, () => {
			const parsed = schemaCombo.parse(
				combo({ models: testCase.refs.map((ref, priority) => ({ ref, priority })) })
			);

			expect(comboModelSummary(parsed)).toBe(testCase.expected);
		});
	}
});
