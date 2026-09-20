// Token saver draft-helper tests (docs/SPEC-UI/001-SPEC-UI.md §6.7).
//
// The rules under test are the ones a per-section save needs against a whole-document route: an edit
// cannot mutate the document it was copied from, a save writes the edited group and leaves the other two
// as they were read, and an empty allowlist reads as every filter rather than as none.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { RTK_FILTER_NAMES, type TokenSaver } from '$lib/schemas/token-saver';
import {
	headroomUrlWarning,
	orderRtkFilters,
	rtkAllowlistLabel,
	tokenSaverCopy,
	tokenSaverSaveDraft,
	tokenSaverSectionDirty,
	toggleRtkFilter,
	unknownRtkFilters
} from '$lib/schemas/token-saver-form';

function document_(overrides: Partial<TokenSaver> = {}): TokenSaver {
	return {
		rtk: { enabled: false, filters: [] },
		headroom: { enabled: false, url: '', compress_user_messages: false },
		ponytail: { enabled: false, level: 'full' },
		...overrides
	};
}

describe('tokenSaverCopy', () => {
	it('copies every group', () => {
		const original = document_({
			rtk: { enabled: true, filters: ['grep'] },
			headroom: { enabled: true, url: 'http://localhost:8787', compress_user_messages: true },
			ponytail: { enabled: true, level: 'ultra' }
		});

		expect(tokenSaverCopy(original)).toEqual(original);
	});

	it('detaches the copy, so an edit cannot mutate the document it came from', () => {
		const original = document_({ rtk: { enabled: true, filters: ['grep'] } });
		const copy = tokenSaverCopy(original);

		copy.rtk.filters.push('ls');
		copy.rtk.enabled = false;
		copy.ponytail.level = 'lite';

		expect(original.rtk.filters).toEqual(['grep']);
		expect(original.rtk.enabled).toBe(true);
		expect(original.ponytail.level).toBe('full');
	});
});

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

describe('tokenSaverSectionDirty', () => {
	const server = document_({ rtk: { enabled: true, filters: ['grep'] } });

	const cases = [
		{
			name: 'rtk is clean against itself',
			section: 'rtk' as const,
			draft: document_({ rtk: { enabled: true, filters: ['grep'] } }),
			expected: false
		},
		{
			name: 'rtk is dirty when the toggle moved',
			section: 'rtk' as const,
			draft: document_({ rtk: { enabled: false, filters: ['grep'] } }),
			expected: true
		},
		{
			name: 'rtk is dirty when the allowlist moved',
			section: 'rtk' as const,
			draft: document_({ rtk: { enabled: true, filters: ['grep', 'ls'] } }),
			expected: true
		},
		{
			name: 'headroom is clean while untouched',
			section: 'headroom' as const,
			draft: document_({ rtk: { enabled: true, filters: ['grep'] } }),
			expected: false
		},
		{
			name: 'headroom is dirty when the URL changed',
			section: 'headroom' as const,
			draft: document_({
				rtk: { enabled: true, filters: ['grep'] },
				headroom: { enabled: false, url: 'http://localhost:8787', compress_user_messages: false }
			}),
			expected: true
		},
		{
			name: 'headroom is dirty when the compress flag changed',
			section: 'headroom' as const,
			draft: document_({
				rtk: { enabled: true, filters: ['grep'] },
				headroom: { enabled: false, url: '', compress_user_messages: true }
			}),
			expected: true
		},
		{
			name: 'ponytail is clean while untouched',
			section: 'ponytail' as const,
			draft: document_({ rtk: { enabled: true, filters: ['grep'] } }),
			expected: false
		},
		{
			name: 'ponytail is dirty when the level changed',
			section: 'ponytail' as const,
			draft: document_({
				rtk: { enabled: true, filters: ['grep'] },
				ponytail: { enabled: false, level: 'ultra' }
			}),
			expected: true
		},
		{
			name: 'ponytail is dirty when the toggle moved',
			section: 'ponytail' as const,
			draft: document_({
				rtk: { enabled: true, filters: ['grep'] },
				ponytail: { enabled: true, level: 'full' }
			}),
			expected: true
		}
	];

	forEachCase(cases, (testCase) => {
		expect(tokenSaverSectionDirty(testCase.section, server, testCase.draft), testCase.name).toBe(
			testCase.expected
		);
	});

	it('treats a section as dirty when there is no document to compare against', () => {
		expect(tokenSaverSectionDirty('rtk', null, document_())).toBe(true);
	});

	it('compares the allowlist in canonical order, so a reordered read is not dirty', () => {
		const reordered = document_({ rtk: { enabled: true, filters: ['ls', 'grep'] } });
		expect(
			tokenSaverSectionDirty(
				'rtk',
				reordered,
				document_({ rtk: { enabled: true, filters: ['grep', 'ls'] } })
			)
		).toBe(false);
	});
});

describe('tokenSaverSaveDraft', () => {
	const server = document_({
		rtk: { enabled: false, filters: [] },
		headroom: { enabled: true, url: 'http://localhost:8787', compress_user_messages: true },
		ponytail: { enabled: true, level: 'ultra' }
	});

	const cases = [
		{
			name: 'a RTK save takes RTK from the draft and the other groups from the last read',
			section: 'rtk' as const,
			draft: {
				...server,
				rtk: { enabled: true, filters: ['grep'] },
				ponytail: { enabled: false, level: 'lite' as const }
			},
			expected: {
				rtk: { enabled: true, filters: ['grep'] },
				headroom: server.headroom,
				ponytail: server.ponytail
			}
		},
		{
			name: 'a Headroom save takes Headroom from the draft and the other groups from the last read',
			section: 'headroom' as const,
			draft: {
				...server,
				headroom: { enabled: false, url: '', compress_user_messages: false },
				rtk: { enabled: true, filters: ['grep'] }
			},
			expected: {
				headroom: { enabled: false, url: '', compress_user_messages: false },
				rtk: server.rtk,
				ponytail: server.ponytail
			}
		},
		{
			name: 'a Ponytail save takes Ponytail from the draft and the other groups from the last read',
			section: 'ponytail' as const,
			draft: {
				...server,
				ponytail: { enabled: false, level: 'lite' as const },
				rtk: { enabled: true, filters: ['grep'] }
			},
			expected: {
				ponytail: { enabled: false, level: 'lite' },
				rtk: server.rtk,
				headroom: server.headroom
			}
		}
	];

	forEachCase(cases, (testCase) => {
		expect(tokenSaverSaveDraft(testCase.section, testCase.draft, server), testCase.name).toEqual(
			testCase.expected
		);
	});

	it('detaches the saved group, so a later draft edit cannot change the merged document', () => {
		const draft = document_({ rtk: { enabled: true, filters: ['grep'] } });
		const merged = tokenSaverSaveDraft('rtk', draft, document_());
		draft.rtk.filters.push('ls');

		expect(merged.rtk.filters).toEqual(['grep']);
	});
});

describe('headroomUrlWarning', () => {
	const cases = [
		{
			name: 'says nothing while the group is off',
			input: { enabled: false, url: '', compress_user_messages: false },
			expected: null
		},
		{
			name: 'says nothing when the group is on with a URL',
			input: { enabled: true, url: 'http://localhost:8787', compress_user_messages: false },
			expected: null
		},
		{
			name: 'warns when the group is on with no URL',
			input: { enabled: true, url: '', compress_user_messages: false },
			expected:
				'No URL is set, so every compression call will be skipped and the request passes through unchanged.'
		},
		{
			name: 'treats a whitespace-only URL as no URL',
			input: { enabled: true, url: '   ', compress_user_messages: false },
			expected:
				'No URL is set, so every compression call will be skipped and the request passes through unchanged.'
		}
	];

	forEachCase(cases, (testCase) => {
		expect(headroomUrlWarning(testCase.input), testCase.name).toBe(testCase.expected);
	});
});
