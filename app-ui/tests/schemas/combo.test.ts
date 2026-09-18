// Combo schema contract tests (docs/SPEC-API/001-SPEC-API.md §7.7, docs/SPEC-UI §6.4).
//
// The strategy is the whole shape of a combo, so these tests pin the rule the API enforces in its domain
// layer: `fallback` ignores both optional fields, `round_robin` requires a sticky limit and refuses a
// judge, and `fusion` requires a judge and refuses a sticky limit. The panel has to reach the same verdict
// before the round trip, and it has to drop the field a strategy ignores, because a leftover value is a
// save the API rejects rather than a value it ignores.

import { describe, expect, it } from 'vitest';
import {
	COMBO_STRATEGIES,
	COMBO_STRATEGY_EXPLANATIONS,
	comboModelSummary,
	comboName,
	comboStrategyLabel,
	duplicateComboRefs,
	reorderComboModels,
	schemaCombo,
	schemaComboList,
	schemaComboModelEntry,
	usesJudgeModel,
	usesStickyLimit,
	type ComboModelEntry
} from '$lib/schemas/combo';
import {
	buildComboBody,
	comboToForm,
	schemaComboForm,
	type ComboForm
} from '$lib/schemas/combo-form';

function combo(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'cmb_1',
		name: 'daily',
		strategy: 'fallback',
		sticky_limit: 1,
		judge_model: '',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		created_at: '2026-09-18T00:00:00Z',
		updated_at: '2026-09-18T00:00:00Z',
		...overrides
	};
}

function form(overrides: Partial<ComboForm> = {}): ComboForm {
	return {
		name: 'daily',
		strategy: 'fallback',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		stickyLimit: 1,
		judgeModel: '',
		...overrides
	};
}

describe('comboName', () => {
	const ACCEPTED: { value: string; expected: string; why: string }[] = [
		{ value: 'daily', expected: 'daily', why: 'a plain name' },
		{ value: 'gpt-4o.fast', expected: 'gpt-4o.fast', why: 'dots, dashes, and digits' },
		{ value: '  spaced  ', expected: 'spaced', why: 'surrounding whitespace is trimmed' },
		{ value: 'a', expected: 'a', why: 'a single character' },
		{ value: 'A_1-b.c', expected: 'A_1-b.c', why: 'mixed case and underscore' }
	];

	for (const testCase of ACCEPTED) {
		it(`accepts ${testCase.why}`, () => {
			const result = comboName.safeParse(testCase.value);

			expect(result.success).toBe(true);
			expect(result.success && result.data).toBe(testCase.expected);
		});
	}

	const REJECTED: { value: string; why: string }[] = [
		{ value: '', why: 'an empty name' },
		{ value: '   ', why: 'a whitespace-only name' },
		{ value: 'openai/gpt-4o', why: 'a slash, which would read as a provider/model reference' },
		{ value: 'two words', why: 'a space inside the name' },
		{ value: 'emoji😀', why: 'a character outside the accepted set' },
		{ value: 'a'.repeat(121), why: 'a name over 120 characters' }
	];

	for (const testCase of REJECTED) {
		it(`rejects ${testCase.why}`, () => {
			expect(comboName.safeParse(testCase.value).success).toBe(false);
		});
	}

	it('accepts a name at exactly the 120 character limit', () => {
		expect(comboName.safeParse('a'.repeat(120)).success).toBe(true);
	});
});

describe('schemaComboModelEntry', () => {
	it('accepts a reference with its priority', () => {
		expect(schemaComboModelEntry.safeParse({ ref: 'openai/gpt-4o', priority: 0 }).success).toBe(
			true
		);
	});

	it('coerces a priority typed as a string, because a number input yields one', () => {
		const result = schemaComboModelEntry.safeParse({ ref: 'openai/gpt-4o', priority: '7' });

		expect(result.success).toBe(true);
		expect(result.success && result.data.priority).toBe(7);
	});

	it('trims the reference', () => {
		const result = schemaComboModelEntry.safeParse({ ref: '  openai/gpt-4o  ', priority: 0 });

		expect(result.success && result.data.ref).toBe('openai/gpt-4o');
	});

	const INVALID: { payload: Record<string, unknown>; why: string }[] = [
		{ payload: { ref: '', priority: 0 }, why: 'an empty reference' },
		{ payload: { ref: '   ', priority: 0 }, why: 'a whitespace-only reference' },
		{ payload: { ref: 'openai/gpt-4o', priority: -1 }, why: 'a negative priority' },
		{ payload: { ref: 'openai/gpt-4o', priority: 100001 }, why: 'a priority past the API maximum' },
		{ payload: { ref: 'openai/gpt-4o', priority: 1.5 }, why: 'a fractional priority' },
		{ payload: { ref: 'a'.repeat(201), priority: 0 }, why: 'a reference over 200 characters' }
	];

	for (const testCase of INVALID) {
		it(`rejects ${testCase.why}`, () => {
			expect(schemaComboModelEntry.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('schemaCombo', () => {
	it('accepts a full combo', () => {
		expect(schemaCombo.safeParse(combo()).success).toBe(true);
	});

	it('accepts models as null, which is how Go marshals a nil slice, and normalizes to empty', () => {
		const result = schemaCombo.safeParse(combo({ models: null }));

		expect(result.success).toBe(true);
		expect(result.success && result.data.models).toEqual([]);
	});

	it('accepts an omitted judge_model, because the field is marked omitempty, and normalizes to empty', () => {
		const payload = combo();
		delete payload.judge_model;

		const result = schemaCombo.safeParse(payload);

		expect(result.success).toBe(true);
		expect(result.success && result.data.judge_model).toBe('');
	});

	it('accepts null judge_model the same way as an absent one', () => {
		const result = schemaCombo.safeParse(combo({ judge_model: null }));

		expect(result.success).toBe(true);
		expect(result.success && result.data.judge_model).toBe('');
	});

	it('accepts omitted timestamps', () => {
		const payload = combo();
		delete payload.created_at;
		delete payload.updated_at;

		expect(schemaCombo.safeParse(payload).success).toBe(true);
	});

	const INVALID: { payload: Record<string, unknown>; why: string }[] = [
		{ payload: combo({ id: '' }), why: 'an empty id' },
		{ payload: combo({ name: '' }), why: 'an empty name' },
		{ payload: combo({ strategy: '' }), why: 'an empty strategy' },
		{ payload: combo({ sticky_limit: 1.5 }), why: 'a fractional sticky limit' },
		{
			payload: combo({ models: [{ ref: '', priority: 0 }] }),
			why: 'a model entry with no reference'
		}
	];

	for (const testCase of INVALID) {
		it(`rejects ${testCase.why}`, () => {
			expect(schemaCombo.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('schemaComboList', () => {
	it('requires the pagination block', () => {
		expect(schemaComboList.safeParse({ data: [combo()] }).success).toBe(false);
	});

	it('accepts an empty page', () => {
		expect(
			schemaComboList.safeParse({ data: [], meta: { page: 1, per_page: 25, total: 0 } }).success
		).toBe(true);
	});
});

describe('strategy predicates', () => {
	const CASES: {
		strategy: string;
		sticky: boolean;
		judge: boolean;
		why: string;
	}[] = [
		{ strategy: 'fallback', sticky: false, judge: false, why: 'fallback reads neither field' },
		{
			strategy: 'round_robin',
			sticky: true,
			judge: false,
			why: 'round_robin reads only the sticky limit'
		},
		{ strategy: 'fusion', sticky: false, judge: true, why: 'fusion reads only the judge model' },
		{
			strategy: 'weighted',
			sticky: false,
			judge: false,
			why: 'a strategy the panel does not know reads neither'
		},
		{ strategy: '', sticky: false, judge: false, why: 'an empty strategy reads neither' }
	];

	for (const testCase of CASES) {
		it(testCase.why, () => {
			expect(usesStickyLimit(testCase.strategy)).toBe(testCase.sticky);
			expect(usesJudgeModel(testCase.strategy)).toBe(testCase.judge);
		});
	}

	it('covers every strategy the panel offers with an explanation and a label', () => {
		// A strategy without copy would render an empty select option and an empty description, so the two
		// vocabularies are asserted to agree rather than left to review.
		for (const strategy of COMBO_STRATEGIES) {
			expect(COMBO_STRATEGY_EXPLANATIONS[strategy], `${strategy} has no explanation`).toBeTruthy();
			expect(comboStrategyLabel(strategy)).not.toBe(strategy);
		}
	});
});

describe('duplicateComboRefs', () => {
	const CASES: { refs: string[]; expected: string[]; why: string }[] = [
		{ refs: [], expected: [], why: 'an empty list' },
		{ refs: ['a/b'], expected: [], why: 'a single reference' },
		{ refs: ['a/b', 'c/d'], expected: [], why: 'two distinct references' },
		{ refs: ['a/b', 'a/b'], expected: ['a/b'], why: 'one reference listed twice' },
		{ refs: ['a/b', 'a/b', 'a/b'], expected: ['a/b'], why: 'one reference listed three times' },
		{
			refs: ['a/b', 'c/d', 'a/b', 'c/d'],
			expected: ['a/b', 'c/d'],
			why: 'two references each listed twice, reported once each'
		},
		{
			refs: ['  a/b  ', 'a/b'],
			expected: ['a/b'],
			why: 'whitespace does not make two references distinct'
		},
		{ refs: ['A/b', 'a/b'], expected: [], why: 'references are case-sensitive identifiers' },
		{
			refs: ['', ''],
			expected: [],
			why: 'blank references are ignored, because they fail validation anyway'
		},
		{ refs: ['   ', 'a/b'], expected: [], why: 'a whitespace-only reference is ignored too' }
	];

	for (const testCase of CASES) {
		it(`returns ${JSON.stringify(testCase.expected)} for ${testCase.why}`, () => {
			expect(duplicateComboRefs(testCase.refs.map((ref) => ({ ref })))).toEqual(testCase.expected);
		});
	}
});

describe('reorderComboModels', () => {
	const entries: ComboModelEntry[] = [
		{ ref: 'a/1', priority: 0 },
		{ ref: 'b/2', priority: 1 },
		{ ref: 'c/3', priority: 2 },
		{ ref: 'd/4', priority: 3 }
	];

	const CASES: { from: number; to: number; expected: string[]; why: string }[] = [
		{ from: 0, to: 0, expected: ['a/1', 'b/2', 'c/3', 'd/4'], why: 'a move onto itself' },
		{ from: 0, to: 1, expected: ['b/2', 'a/1', 'c/3', 'd/4'], why: 'one step down' },
		{ from: 1, to: 0, expected: ['b/2', 'a/1', 'c/3', 'd/4'], why: 'one step up' },
		{
			from: 0,
			to: 3,
			expected: ['b/2', 'c/3', 'd/4', 'a/1'],
			why: 'the first entry moved to the end'
		},
		{
			from: 3,
			to: 0,
			expected: ['d/4', 'a/1', 'b/2', 'c/3'],
			why: 'the last entry moved to the start'
		},
		{
			from: 2,
			to: 1,
			expected: ['a/1', 'c/3', 'b/2', 'd/4'],
			why: 'a middle entry moved one step up'
		},
		{ from: -1, to: 2, expected: ['a/1', 'b/2', 'c/3', 'd/4'], why: 'a negative source index' },
		{ from: 1, to: 9, expected: ['a/1', 'b/2', 'c/3', 'd/4'], why: 'a destination past the end' },
		{ from: 9, to: 0, expected: ['a/1', 'b/2', 'c/3', 'd/4'], why: 'a source past the end' }
	];

	for (const testCase of CASES) {
		it(`returns ${testCase.expected.join(', ')} for ${testCase.why}`, () => {
			const result = reorderComboModels(entries, testCase.from, testCase.to);

			expect(result.map((entry) => entry.ref)).toEqual(testCase.expected);
			expect(result.map((entry) => entry.priority)).toEqual(testCase.expected.map((_, i) => i));
		});
	}

	it('renumbers every priority so the numbers describe the positions', () => {
		// Swapping two priorities would leave the order right and the numbers jumping, so the next move
		// would be computed against numbers that no longer describe the positions.
		const result = reorderComboModels(entries, 3, 0);

		expect(result.map((entry) => entry.priority)).toEqual([0, 1, 2, 3]);
	});

	it('leaves the input list untouched', () => {
		const original = entries.map((entry) => ({ ...entry }));
		reorderComboModels(entries, 0, 3);

		expect(entries).toEqual(original);
	});

	it('handles an empty list and a single entry', () => {
		expect(reorderComboModels([], 0, 0)).toEqual([]);
		expect(reorderComboModels([{ ref: 'a/1', priority: 0 }], 0, 0)).toEqual([
			{ ref: 'a/1', priority: 0 }
		]);
	});

	it('keeps a list that arrived with non-contiguous priorities contiguous', () => {
		const sparse: ComboModelEntry[] = [
			{ ref: 'a/1', priority: 50 },
			{ ref: 'b/2', priority: 900 }
		];

		expect(reorderComboModels(sparse, 0, 0).map((entry) => entry.priority)).toEqual([0, 1]);
	});
});

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
