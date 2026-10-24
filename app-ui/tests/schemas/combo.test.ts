// Combo schema contract tests (docs/SPEC-API/001-SPEC-API.md §7.7, docs/SPEC-UI §6.4).
//
// The strategy is the whole shape of a combo, so these tests pin the rule the API enforces in its domain
// layer: `fallback` ignores both optional fields, `round_robin` requires a sticky limit and refuses a
// judge, and `fusion` requires a judge and refuses a sticky limit. The panel has to reach the same verdict
// before the round trip, and it has to drop the field a strategy ignores, because a leftover value is a
// save the API rejects rather than a value it ignores.
//
// The strategy predicates, the duplicate check and the reorder are in `combo-order.test.ts`; the editor's own
// form and the body it writes are in `combo-form.test.ts` and `combo-body.test.ts`.

import { describe, expect, it } from 'vitest';
import { comboName, schemaCombo, schemaComboList, schemaComboModelEntry } from '$lib/schemas/combo';
import { combo } from '../support/combo-fixtures';
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
