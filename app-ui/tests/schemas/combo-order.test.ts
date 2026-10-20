// How a combo's model list is ordered, and what the panel refuses to save (docs/SPEC-API/001-SPEC-API.md §7.6).
//
// The row schemas are in `combo.test.ts`. These three are the operations on a list: the strategy predicates
// that decide whether a combo can be dialled at all, the duplicate check, and the reorder a drag hands back.

import { describe, expect, it } from 'vitest';
import {
	COMBO_STRATEGIES,
	COMBO_STRATEGY_EXPLANATIONS,
	comboStrategyLabel,
	duplicateComboRefs,
	reorderComboModels,
	usesJudgeModel,
	usesStickyLimit,
	type ComboModelEntry
} from '$lib/schemas/combo';
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
