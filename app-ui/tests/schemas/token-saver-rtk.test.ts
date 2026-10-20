// The RTK allowlist helpers (docs/SPEC-UI/001-SPEC-UI.md §6.7).
//
// The allowlist is stored as a plain string array, so a name the panel does not know is a value that has
// to survive a round trip rather than a parse failure. These four functions are where that survives or is
// lost: the order the screen shows, a single toggle, the names it cannot offer, and the sentence that
// counts them.

import { describe, expect } from 'vitest';
import { forEachCase } from '../support/tables';
import { RTK_FILTER_NAMES } from '$lib/schemas/token-saver';
import {
	orderRtkFilters,
	rtkAllowlistLabel,
	toggleRtkFilter,
	unknownRtkFilters
} from '$lib/schemas/token-saver-form';

describe('orderRtkFilters', () => {
	const cases = [
		{
			name: 'keeps the canonical order',
			input: ['grep', 'git-diff'],
			expected: ['git-diff', 'grep']
		},
		{ name: 'leaves an empty list empty', input: [] as string[], expected: [] },
		{
			name: 'keeps an unknown name after the known ones',
			input: ['future-filter', 'grep'],
			expected: ['grep', 'future-filter']
		},
		{
			name: 'drops a duplicate',
			input: ['grep', 'grep', 'git-log'],
			expected: ['git-log', 'grep']
		},
		{
			name: 'keeps two unknown names in the order they arrived',
			input: ['zeta', 'alpha'],
			expected: ['zeta', 'alpha']
		},
		{
			name: 'orders every canonical name as the constant declares it',
			input: [...RTK_FILTER_NAMES].reverse(),
			expected: RTK_FILTER_NAMES
		}
	];

	forEachCase(cases, (testCase) => {
		expect(orderRtkFilters(testCase.input), testCase.name).toEqual(testCase.expected);
	});
});

describe('toggleRtkFilter', () => {
	const cases = [
		{
			name: 'adds a name that is absent',
			input: [] as string[],
			filter: 'grep',
			on: true,
			expected: ['grep']
		},
		{
			name: 'removes a name that is present',
			input: ['grep', 'git-log'],
			filter: 'grep',
			on: false,
			expected: ['git-log']
		},
		{
			name: 'adding twice is the same as adding once',
			input: ['grep'],
			filter: 'grep',
			on: true,
			expected: ['grep']
		},
		{
			name: 'removing an absent name changes nothing',
			input: ['grep'],
			filter: 'ls',
			on: false,
			expected: ['grep']
		},
		{
			name: 'keeps an unknown name while toggling a known one',
			input: ['future'],
			filter: 'ls',
			on: true,
			expected: ['ls', 'future']
		}
	];

	forEachCase(cases, (testCase) => {
		expect(toggleRtkFilter(testCase.input, testCase.filter, testCase.on), testCase.name).toEqual(
			testCase.expected
		);
	});
});

describe('unknownRtkFilters', () => {
	const cases = [
		{ name: 'reports nothing for an empty list', input: [] as string[], expected: [] },
		{ name: 'reports nothing when every name is canonical', input: ['grep', 'ls'], expected: [] },
		{
			name: 'reports the names the panel cannot offer',
			input: ['grep', 'future'],
			expected: ['future']
		},
		{ name: 'reports one entry per name', input: ['future', 'future'], expected: ['future'] }
	];

	forEachCase(cases, (testCase) => {
		expect(unknownRtkFilters(testCase.input), testCase.name).toEqual(testCase.expected);
	});
});

describe('rtkAllowlistLabel', () => {
	const cases = [
		{
			name: 'reads an empty allowlist as every filter',
			input: [] as string[],
			expected: 'Every filter is eligible.'
		},
		{
			name: 'counts a partial allowlist against the twelve',
			input: ['grep', 'ls', 'tree'],
			expected: '3 of 12 filters allowed.'
		},
		{
			name: 'counts a full allowlist as twelve',
			input: RTK_FILTER_NAMES,
			expected: '12 of 12 filters allowed.'
		},
		{
			name: 'counts a stored name the panel does not offer',
			input: ['future'],
			expected: '1 of 12 filters allowed.'
		}
	];

	forEachCase(cases, (testCase) => {
		expect(rtkAllowlistLabel(testCase.input), testCase.name).toBe(testCase.expected);
	});
});
