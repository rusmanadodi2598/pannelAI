// What the vision adapter tab says about its own state (docs/SPEC-UI/001-SPEC-UI.md §6.4 tab 2).
//
// Both sentences are derived from one document rather than written at the call site, and both are pinned
// because each one is the only thing that names a state the control cannot show: a warning for "enabled with
// nothing selected", which looks configured and adapts nothing, and a status line that says round-robin is
// off when it is off rather than leaving the row to be read as broken.

import { describe, expect, it } from 'vitest';
import { visionAdapterStatusText, visionAdapterWarning } from '$lib/schemas/vision-adapter';

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
			expected: 'On, but no model selected. Pick one to adapt images.',
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
