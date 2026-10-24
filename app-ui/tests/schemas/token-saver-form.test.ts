// Token saver draft-helper tests (docs/SPEC-UI/001-SPEC-UI.md §6.7).
//
// The rules under test are the ones a per-section save needs against a whole-document route: an edit
// cannot mutate the document it was copied from, a save writes the edited group and leaves the other two
// as they were read, and a group that is on with nothing to point at says so. The allowlist helpers the
// same screen uses are in `token-saver-rtk.test.ts`.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { tokenSaverDocument } from '../support/token-saver-stub';
import {
	headroomUrlWarning,
	tokenSaverCopy,
	tokenSaverSaveDraft,
	tokenSaverSectionDirty
} from '$lib/schemas/token-saver-form';

describe('tokenSaverCopy', () => {
	it('copies every group', () => {
		const original = tokenSaverDocument({
			rtk: { enabled: true, filters: ['grep'] },
			headroom: { enabled: true, url: 'http://localhost:8787', compress_user_messages: true },
			ponytail: { enabled: true, level: 'ultra' }
		});

		expect(tokenSaverCopy(original)).toEqual(original);
	});

	it('detaches the copy, so an edit cannot mutate the document it came from', () => {
		const original = tokenSaverDocument({ rtk: { enabled: true, filters: ['grep'] } });
		const copy = tokenSaverCopy(original);

		copy.rtk.filters.push('ls');
		copy.rtk.enabled = false;
		copy.ponytail.level = 'lite';

		expect(original.rtk.filters).toEqual(['grep']);
		expect(original.rtk.enabled).toBe(true);
		expect(original.ponytail.level).toBe('full');
	});
});

describe('tokenSaverSectionDirty', () => {
	const server = tokenSaverDocument({ rtk: { enabled: true, filters: ['grep'] } });

	const cases = [
		{
			name: 'rtk is clean against itself',
			section: 'rtk' as const,
			draft: tokenSaverDocument({ rtk: { enabled: true, filters: ['grep'] } }),
			expected: false
		},
		{
			name: 'rtk is dirty when the toggle moved',
			section: 'rtk' as const,
			draft: tokenSaverDocument({ rtk: { enabled: false, filters: ['grep'] } }),
			expected: true
		},
		{
			name: 'rtk is dirty when the allowlist moved',
			section: 'rtk' as const,
			draft: tokenSaverDocument({ rtk: { enabled: true, filters: ['grep', 'ls'] } }),
			expected: true
		},
		{
			name: 'headroom is clean while untouched',
			section: 'headroom' as const,
			draft: tokenSaverDocument({ rtk: { enabled: true, filters: ['grep'] } }),
			expected: false
		},
		{
			name: 'headroom is dirty when the URL changed',
			section: 'headroom' as const,
			draft: tokenSaverDocument({
				rtk: { enabled: true, filters: ['grep'] },
				headroom: { enabled: false, url: 'http://localhost:8787', compress_user_messages: false }
			}),
			expected: true
		},
		{
			name: 'headroom is dirty when the compress flag changed',
			section: 'headroom' as const,
			draft: tokenSaverDocument({
				rtk: { enabled: true, filters: ['grep'] },
				headroom: { enabled: false, url: '', compress_user_messages: true }
			}),
			expected: true
		},
		{
			name: 'ponytail is clean while untouched',
			section: 'ponytail' as const,
			draft: tokenSaverDocument({ rtk: { enabled: true, filters: ['grep'] } }),
			expected: false
		},
		{
			name: 'ponytail is dirty when the level changed',
			section: 'ponytail' as const,
			draft: tokenSaverDocument({
				rtk: { enabled: true, filters: ['grep'] },
				ponytail: { enabled: false, level: 'ultra' }
			}),
			expected: true
		},
		{
			name: 'ponytail is dirty when the toggle moved',
			section: 'ponytail' as const,
			draft: tokenSaverDocument({
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
		expect(tokenSaverSectionDirty('rtk', null, tokenSaverDocument())).toBe(true);
	});

	it('compares the allowlist in canonical order, so a reordered read is not dirty', () => {
		const reordered = tokenSaverDocument({ rtk: { enabled: true, filters: ['ls', 'grep'] } });
		expect(
			tokenSaverSectionDirty(
				'rtk',
				reordered,
				tokenSaverDocument({ rtk: { enabled: true, filters: ['grep', 'ls'] } })
			)
		).toBe(false);
	});
});

describe('tokenSaverSaveDraft', () => {
	const server = tokenSaverDocument({
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
		const draft = tokenSaverDocument({ rtk: { enabled: true, filters: ['grep'] } });
		const merged = tokenSaverSaveDraft('rtk', draft, tokenSaverDocument());
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
			expected: 'No URL set, so nothing is compressed.'
		},
		{
			name: 'treats a whitespace-only URL as no URL',
			input: { enabled: true, url: '   ', compress_user_messages: false },
			expected: 'No URL set, so nothing is compressed.'
		}
	];

	forEachCase(cases, (testCase) => {
		expect(headroomUrlWarning(testCase.input), testCase.name).toBe(testCase.expected);
	});
});
