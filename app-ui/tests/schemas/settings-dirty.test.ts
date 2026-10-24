// The dirty answer every settings tab shows beside its discard action (docs/SPEC-UI/001-SPEC-UI.md §6.13).
//
// The indicator and the action read one answer, so these cases are about the answer itself: whether the
// draft still differs from the group that was loaded.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { settingsGroupDirty } from '$lib/schemas/settings-dirty';

describe('settingsGroupDirty', () => {
	const loaded = { retention_days: 7, request_capture_enabled: false };

	const cases = [
		{ name: 'is false for an identical group', input: { ...loaded }, expected: false },
		{
			name: 'is true when a value changed',
			input: { ...loaded, retention_days: 30 },
			expected: true
		},
		{
			name: 'is true when a boolean changed',
			input: { ...loaded, request_capture_enabled: true },
			expected: true
		},
		{
			name: 'is false again once the change is reverted',
			input: { retention_days: 7, request_capture_enabled: false },
			expected: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(settingsGroupDirty(loaded, testCase.input), testCase.name).toBe(testCase.expected);
	});

	it('treats a missing loaded group as dirty, because there is nothing to compare against', () => {
		expect(settingsGroupDirty(null, loaded)).toBe(true);
	});

	it('compares by key, not by insertion order', () => {
		expect(settingsGroupDirty({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(false);
	});

	it('reports dirty when the draft carries a key the loaded group does not', () => {
		expect(settingsGroupDirty({ a: 1 }, { a: 1, b: 2 })).toBe(true);
	});
});
